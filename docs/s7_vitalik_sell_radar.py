#!/usr/bin/env python3
"""
Vitalik Sell Radar — Event-Driven Edition
WebSocket subscription to ERC-20 Transfer events, real-time detection of
Vitalik's sell activity, instant push to Telegram.

Architecture:
1. WebSocket subscribes to Transfer(from=vitalik) events → sub-second detection
2. Classifies sell behavior (transfers to DEX Router / CEX / LP Pool)
3. Queries token info + price via DexScreener
4. Pushes alert to Telegram
5. Auto-reconnect + multi-RPC failover
"""

import asyncio
import json
import logging
import os
import signal
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

import aiohttp
import websockets

# Load .env file
def load_env():
    env_path = Path(__file__).parent / ".env"
    if env_path.exists():
        for line in env_path.read_text().splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                os.environ.setdefault(k.strip(), v.strip())

load_env()

# ============================================================
# Configuration
# ============================================================

# Vitalik's main wallet
VITALIK_ADDRESS = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
VITALIK_PADDED = "0x" + VITALIK_ADDRESS[2:].lower().zfill(64)

# ERC-20 Transfer event topic
TRANSFER_TOPIC = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

# WebSocket RPC endpoints (free, support eth_subscribe)
WS_ENDPOINTS = [
    "wss://ethereum-rpc.publicnode.com",
    "wss://eth.drpc.org",
    "wss://ethereum.publicnode.com",
]

# HTTP RPC for querying token info
HTTP_RPC = os.environ.get("HTTP_RPC", "https://eth.drpc.org")

# Telegram
TG_BOT_TOKEN = os.environ.get("TG_BOT_TOKEN", "")
TG_CHAT_ID = os.environ.get("TG_CHAT_ID", "")

# Known DEX Router addresses (sell destinations)
KNOWN_DEX_ROUTERS = {
    # Uniswap
    "0x7a250d5630b4cf539739df2c5dacb4c659f2488d": "Uniswap V2 Router",
    "0xe592427a0aece92de3edee1f18e0157c05861564": "Uniswap V3 Router",
    "0x68b3465833fb72a70ecdf485e0e4c7bd8665fc45": "Uniswap V3 Router2",
    "0x3fc91a3afd70395cd496c647d5a6cc9d4b2b7fad": "Uniswap Universal Router",
    "0xef1c6e67703c7bd7107eed8303fbe6ec2554bf6b": "Uniswap Universal Router (old)",
    # 1inch
    "0x1111111254eeb25477b68fb85ed929f73a960582": "1inch V5",
    "0x111111125421ca6dc452d289314280a0f8842a65": "1inch V6",
    # SushiSwap
    "0xd9e1ce17f2641f24ae83637ab66a2cca9c378b9f": "SushiSwap Router",
    # CoW Protocol
    "0x9008d19f58aabd9ed0d60971565aa8510560ab41": "CoW Settlement",
    # 0x
    "0xdef1c0ded9bec7f1a1670819833240f027b25eff": "0x Exchange Proxy",
    # Curve
    "0x99a58482bd75cbab83b27ec03ca68ff489b5788f": "Curve Router",
}

# Known CEX hot wallets (partial list)
KNOWN_CEX = {
    "0x28c6c06298d514db089934071355e5743bf21d60": "Binance Hot Wallet",
    "0x21a31ee1afc51d94c2efccaa2092ad1028285549": "Binance Hot Wallet 2",
    "0xdfd5293d8e347dfe59e90efd55b2956a1343963d": "Binance Hot Wallet 3",
    "0x56eddb7aa87536c09ccc2793473599fd21a8b17f": "Binance Hot Wallet 4",
    "0x71660c4005ba85c37ccec55d0c4493e66fe775d3": "Coinbase",
    "0xa9d1e08c7793af67e9d92fe308d5697fb81d3e43": "Coinbase 10",
    "0x503828976d22510aad0201ac7ec88293211d23da": "Coinbase 2",
    "0x2faf487a4414fe77e2327f0bf4ae2a264a776ad2": "FTX (defunct)",
    "0x267be1c1d684f78cb4f6a176c4911b741e4ffdc0": "Kraken",
    "0xae2d4617c862309a3d75a0ffb358c7a5009c673f": "Kraken 10",
}

# Minimum notification amount (USD), 0 = notify all
MIN_NOTIFY_USD = int(os.environ.get("MIN_NOTIFY_USD", "0"))

# Reconnect settings
RECONNECT_DELAY = 5
MAX_RECONNECT_DELAY = 60

# ============================================================
# Logging
# ============================================================

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
log = logging.getLogger("vitalik-radar")

# ============================================================
# Global state
# ============================================================

# Dedup set for processed tx hashes (last 1000)
seen_txs: set = set()
seen_txs_list: list = []

# HTTP session
http_session: aiohttp.ClientSession | None = None

# Token info cache: {address: {symbol, name, decimals}}
token_cache: dict = {}

# Running flag
running = True


# ============================================================
# Utility functions
# ============================================================

def shorten_addr(addr: str) -> str:
    """Shorten address for display"""
    return f"{addr[:6]}...{addr[-4:]}"


def decode_transfer_value(data_hex: str, decimals: int) -> float:
    """Decode Transfer event value"""
    try:
        raw = int(data_hex, 16)
        return raw / (10 ** decimals)
    except:
        return 0.0


def topic_to_address(topic: str) -> str:
    """Extract 20-byte address from 32-byte topic"""
    return "0x" + topic[-40:]


def classify_recipient(to_addr: str) -> tuple[str, str]:
    """
    Classify recipient address.
    Returns (type, name)
    Type: "dex" / "cex" / "pool" / "unknown"
    """
    to_lower = to_addr.lower()

    if to_lower in KNOWN_DEX_ROUTERS:
        return "dex", KNOWN_DEX_ROUTERS[to_lower]

    if to_lower in KNOWN_CEX:
        return "cex", KNOWN_CEX[to_lower]

    return "unknown", ""


async def get_http_session() -> aiohttp.ClientSession:
    global http_session
    if http_session is None or http_session.closed:
        http_session = aiohttp.ClientSession()
    return http_session


async def rpc_call(method: str, params: list) -> dict | None:
    """HTTP JSON-RPC call"""
    session = await get_http_session()
    try:
        async with session.post(
            HTTP_RPC,
            json={"jsonrpc": "2.0", "id": 1, "method": method, "params": params},
            timeout=aiohttp.ClientTimeout(total=10),
        ) as resp:
            data = await resp.json()
            return data.get("result")
    except Exception as e:
        log.error(f"RPC call {method} failed: {e}")
        return None


async def get_token_info(token_addr: str) -> dict:
    """Query ERC-20 token info (symbol, name, decimals)"""
    addr_lower = token_addr.lower()
    if addr_lower in token_cache:
        return token_cache[addr_lower]

    info = {"symbol": "???", "name": "Unknown", "decimals": 18, "address": token_addr}

    # symbol()
    result = await rpc_call("eth_call", [
        {"to": token_addr, "data": "0x95d89b41"}, "latest"
    ])
    if result and len(result) > 2:
        try:
            hex_str = result[2:]
            if len(hex_str) >= 128:
                offset = int(hex_str[:64], 16) * 2
                length = int(hex_str[offset:offset+64], 16)
                symbol_hex = hex_str[offset+64:offset+64+length*2]
                info["symbol"] = bytes.fromhex(symbol_hex).decode("utf-8", errors="replace").strip('\x00')
            elif len(hex_str) == 64:
                info["symbol"] = bytes.fromhex(hex_str).decode("utf-8", errors="replace").strip('\x00')
        except:
            pass

    # decimals()
    result = await rpc_call("eth_call", [
        {"to": token_addr, "data": "0x313ce567"}, "latest"
    ])
    if result and len(result) > 2:
        try:
            info["decimals"] = int(result, 16)
        except:
            pass

    # name()
    result = await rpc_call("eth_call", [
        {"to": token_addr, "data": "0x06fdde03"}, "latest"
    ])
    if result and len(result) > 2:
        try:
            hex_str = result[2:]
            if len(hex_str) >= 128:
                offset = int(hex_str[:64], 16) * 2
                length = int(hex_str[offset:offset+64], 16)
                name_hex = hex_str[offset+64:offset+64+length*2]
                info["name"] = bytes.fromhex(name_hex).decode("utf-8", errors="replace").strip('\x00')
            elif len(hex_str) == 64:
                info["name"] = bytes.fromhex(hex_str).decode("utf-8", errors="replace").strip('\x00')
        except:
            pass

    token_cache[addr_lower] = info
    return info


async def check_if_pool(addr: str) -> bool:
    """Check if address is a Uniswap V2/V3 pool (has token0 method)"""
    result = await rpc_call("eth_call", [
        {"to": addr, "data": "0x0dfe1681"}, "latest"  # token0()
    ])
    if result and len(result) == 66:  # 0x + 64 hex chars
        return True
    return False


async def get_eth_price() -> float:
    """Get ETH price from CoinGecko"""
    session = await get_http_session()
    try:
        async with session.get(
            "https://api.coingecko.com/api/v3/simple/price?ids=ethereum&vs_currencies=usd",
            timeout=aiohttp.ClientTimeout(total=5),
        ) as resp:
            data = await resp.json()
            return data.get("ethereum", {}).get("usd", 0)
    except:
        return 2300  # fallback


async def get_token_price_usd(token_addr: str) -> float | None:
    """Get token USD price from DexScreener (free, no API key needed)"""
    session = await get_http_session()
    try:
        async with session.get(
            f"https://api.dexscreener.com/latest/dex/tokens/{token_addr}",
            timeout=aiohttp.ClientTimeout(total=5),
        ) as resp:
            data = await resp.json()
            pairs = data.get("pairs", [])
            if pairs:
                # Pick pair with highest liquidity
                pairs.sort(key=lambda p: float(p.get("liquidity", {}).get("usd", 0) or 0), reverse=True)
                price = float(pairs[0].get("priceUsd", 0) or 0)
                return price if price > 0 else None
    except:
        pass
    return None


# ============================================================
# Telegram notifications
# ============================================================

async def send_telegram(text: str):
    """Send Telegram message"""
    if not TG_BOT_TOKEN:
        log.warning("[TG] No bot token configured, skipping notification")
        return

    session = await get_http_session()
    url = f"https://api.telegram.org/bot{TG_BOT_TOKEN}/sendMessage"
    payload = {
        "chat_id": TG_CHAT_ID,
        "text": text,
        "parse_mode": "HTML",
        "disable_web_page_preview": True,
    }

    try:
        async with session.post(url, json=payload, timeout=aiohttp.ClientTimeout(total=10)) as resp:
            result = await resp.json()
            if not result.get("ok"):
                log.error(f"[TG] Send failed: {result.get('description', '')}")
                # Retry with plain text if HTML parse fails
                if "parse" in result.get("description", "").lower():
                    payload["parse_mode"] = None
                    async with session.post(url, json=payload) as resp2:
                        pass
            else:
                log.info("[TG] Message sent")
    except Exception as e:
        log.error(f"[TG] Error: {e}")


# ============================================================
# Event handling
# ============================================================

async def handle_transfer_event(log_entry: dict):
    """Process a single Transfer event"""
    tx_hash = log_entry.get("transactionHash", "")
    token_addr = log_entry.get("address", "")
    topics = log_entry.get("topics", [])
    data = log_entry.get("data", "0x0")

    # Dedup
    dedup_key = f"{tx_hash}:{token_addr}"
    if dedup_key in seen_txs:
        return
    seen_txs.add(dedup_key)
    seen_txs_list.append(dedup_key)
    # Cleanup (keep last 1000)
    while len(seen_txs_list) > 1000:
        old = seen_txs_list.pop(0)
        seen_txs.discard(old)

    # Parse topics: [Transfer, from, to]
    if len(topics) < 3:
        return

    from_addr = topic_to_address(topics[1])
    to_addr = topic_to_address(topics[2])

    # Confirm it's from Vitalik
    if from_addr.lower() != VITALIK_ADDRESS.lower():
        return

    # Get token info
    token_info = await get_token_info(token_addr)
    symbol = token_info["symbol"]
    decimals = token_info["decimals"]
    amount = decode_transfer_value(data, decimals)

    if amount == 0:
        return

    # Classify recipient
    recipient_type, recipient_name = classify_recipient(to_addr)

    # If unknown, check if it's an LP pool
    is_pool = False
    if recipient_type == "unknown":
        is_pool = await check_if_pool(to_addr)
        if is_pool:
            recipient_type = "pool"
            recipient_name = "LP Pool"

    # Determine if this is a "sell" action
    is_sell = recipient_type in ("dex", "cex", "pool")

    # If not a sell, just a regular transfer — log silently
    if not is_sell:
        log.info(f"[Transfer] {symbol} {amount:,.2f} → {shorten_addr(to_addr)} (regular transfer, not notifying)")
        return

    # Get price
    token_price = await get_token_price_usd(token_addr)
    usd_value = amount * token_price if token_price else None

    # Below minimum notification amount — skip
    if usd_value is not None and usd_value < MIN_NOTIFY_USD:
        log.info(f"[Sell] {symbol} ${usd_value:.0f} < ${MIN_NOTIFY_USD} minimum, skipping")
        return

    # Build notification message
    timestamp = datetime.now(timezone.utc).strftime("%H:%M:%S UTC")

    sell_type_label = {
        "dex": "🔄 DEX Sell",
        "cex": "🏦 CEX Transfer",
        "pool": "🌊 Pool Sell",
    }

    msg_lines = [
        f"<b>🚨 Vitalik Sell Signal</b>",
        f"",
        f"Token: <b>{symbol}</b>",
        f"Amount: {amount:,.4f}",
    ]

    if usd_value is not None:
        msg_lines.append(f"Value: <b>${usd_value:,.0f}</b>")

    if token_price is not None:
        msg_lines.append(f"Price: ${token_price:,.8f}")

    msg_lines.extend([
        f"",
        f"Type: {sell_type_label.get(recipient_type, 'Unknown')}",
        f"Destination: {recipient_name or shorten_addr(to_addr)}",
        f"Time: {timestamp}",
        f"",
        f"TX: https://etherscan.io/tx/{tx_hash}",
        f"Token: https://dexscreener.com/ethereum/{token_addr}",
    ])

    msg = "\n".join(msg_lines)
    log.info(f"[SELL DETECTED] {symbol} {amount:,.4f} → {recipient_name or to_addr} | ${usd_value or '?'}")
    await send_telegram(msg)


# ============================================================
# WebSocket listener main loop
# ============================================================

async def subscribe_and_listen(ws_url: str):
    """Connect to WebSocket and subscribe to Vitalik Transfer events"""
    log.info(f"Connecting to {ws_url}...")

    async with websockets.connect(
        ws_url,
        ping_interval=20,
        ping_timeout=30,
        close_timeout=10,
        max_size=2**20,  # 1MB
    ) as ws:
        # Subscribe to Transfer FROM Vitalik
        sub_from = {
            "jsonrpc": "2.0", "id": 1,
            "method": "eth_subscribe",
            "params": ["logs", {
                "topics": [TRANSFER_TOPIC, VITALIK_PADDED]
            }]
        }
        await ws.send(json.dumps(sub_from))
        resp = await asyncio.wait_for(ws.recv(), timeout=10)
        data = json.loads(resp)
        if "error" in data:
            raise Exception(f"Subscribe failed: {data['error']}")
        sub_id_from = data.get("result", "")
        log.info(f"✅ Subscribed to Transfer FROM Vitalik (id={sub_id_from[:10]}...)")

        # Also subscribe to Transfer TO Vitalik (monitor buys/receives)
        sub_to = {
            "jsonrpc": "2.0", "id": 2,
            "method": "eth_subscribe",
            "params": ["logs", {
                "topics": [TRANSFER_TOPIC, None, VITALIK_PADDED]
            }]
        }
        await ws.send(json.dumps(sub_to))
        resp2 = await asyncio.wait_for(ws.recv(), timeout=10)
        data2 = json.loads(resp2)
        sub_id_to = data2.get("result", "")
        if sub_id_to:
            log.info(f"✅ Subscribed to Transfer TO Vitalik (id={sub_id_to[:10]}...)")

        log.info("🔍 Monitoring Vitalik wallet... waiting for events")

        # Listen for events
        async for message in ws:
            if not running:
                break

            try:
                evt = json.loads(message)

                if "params" not in evt:
                    continue

                sub_id = evt["params"].get("subscription", "")
                log_entry = evt["params"].get("result", {})

                if sub_id == sub_id_from:
                    # Vitalik sent tokens → check if it's a sell
                    await handle_transfer_event(log_entry)
                elif sub_id == sub_id_to:
                    # Vitalik received tokens — log silently
                    token_addr = log_entry.get("address", "")
                    token_info = await get_token_info(token_addr)
                    data_hex = log_entry.get("data", "0x0")
                    amount = decode_transfer_value(data_hex, token_info["decimals"])
                    log.debug(f"[Receive] {token_info['symbol']} +{amount:,.4f}")

            except json.JSONDecodeError:
                continue
            except Exception as e:
                log.error(f"Event handling error: {e}", exc_info=True)


async def main():
    """Main loop with auto-reconnect"""
    global running

    # Validate required config
    if not TG_BOT_TOKEN:
        log.warning("TG_BOT_TOKEN not set — Telegram notifications disabled")
    if not TG_CHAT_ID:
        log.warning("TG_CHAT_ID not set — Telegram notifications disabled")

    # Signal handling
    def shutdown(sig, frame):
        global running
        log.info(f"Received signal {sig}, shutting down...")
        running = False

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    # Startup notice
    log.info("=" * 50)
    log.info("Vitalik Sell Radar — Started")
    log.info(f"Monitoring: {VITALIK_ADDRESS}")
    log.info(f"Min notify: ${MIN_NOTIFY_USD}")
    log.info(f"Telegram: {'✅ Configured' if TG_BOT_TOKEN else '❌ Not configured'}")
    log.info("=" * 50)

    if TG_BOT_TOKEN and TG_CHAT_ID:
        await send_telegram(
            "🟢 Vitalik Sell Radar — Started\n\n"
            f"Monitoring: {shorten_addr(VITALIK_ADDRESS)}\n"
            f"Min notify: ${MIN_NOTIFY_USD}\n"
            "Mode: WebSocket event-driven (sub-second latency)"
        )

    endpoint_idx = 0
    reconnect_delay = RECONNECT_DELAY

    while running:
        ws_url = WS_ENDPOINTS[endpoint_idx % len(WS_ENDPOINTS)]
        try:
            await subscribe_and_listen(ws_url)
            reconnect_delay = RECONNECT_DELAY  # reset on success
        except websockets.exceptions.ConnectionClosed as e:
            log.warning(f"WebSocket closed: {e}")
        except asyncio.TimeoutError:
            log.warning("WebSocket timeout")
        except Exception as e:
            log.error(f"WebSocket error: {e}")

        if not running:
            break

        # Switch endpoint and retry
        endpoint_idx += 1
        log.info(f"Reconnecting in {reconnect_delay}s... (next: {WS_ENDPOINTS[endpoint_idx % len(WS_ENDPOINTS)]})")
        await asyncio.sleep(reconnect_delay)
        reconnect_delay = min(reconnect_delay * 1.5, MAX_RECONNECT_DELAY)

    # Cleanup
    if http_session and not http_session.closed:
        await http_session.close()

    log.info("Vitalik Sell Radar — Stopped")


if __name__ == "__main__":
    asyncio.run(main())
