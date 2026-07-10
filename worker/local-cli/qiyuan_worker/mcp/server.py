"""MCP Server for QIYUAN Browser Worker.

Exposes browser automation tools via the Model Context Protocol (stdio
transport).  The server manages a single BrowserRuntime and lazily
initialises a persistent Playwright browser session on the first tool
call that requires it.

Usage:
    qiyuan-worker mcp [--profile NAME] [--headless]
"""

from __future__ import annotations

import asyncio
import logging
from pathlib import Path

from qiyuan_worker.browser import BrowserRuntime, BrowserRuntimeConfig
from qiyuan_worker.mcp.tools import TOOLS, BrowserToolContext, execute_tool

logger = logging.getLogger(__name__)

SERVER_NAME = "qiyuan-browser"
SERVER_VERSION = "0.1.0"


async def _run_server(
    profile_dir: Path,
    downloads_dir: Path,
    headed: bool,
) -> None:
    """Start the MCP server on stdio."""

    try:
        from mcp.server import Server
        from mcp.server.stdio import stdio_server
        import mcp.types as types
    except ImportError as exc:
        raise RuntimeError(
            "The 'mcp' package is required. Install it with: "
            "pip install 'qiyuan-worker[mcp]'"
        ) from exc

    runtime = BrowserRuntime(
        BrowserRuntimeConfig(
            profile_dir=profile_dir,
            downloads_dir=downloads_dir,
            headed=headed,
        )
    )

    ctx = BrowserToolContext()
    ctx._runtime = runtime

    server = Server(SERVER_NAME)

    @server.list_tools()
    async def list_tools() -> list[types.Tool]:
        return [
            types.Tool(
                name=tool.name,
                description=tool.description,
                inputSchema=tool.input_schema,
            )
            for tool in TOOLS
        ]

    @server.call_tool()
    async def call_tool(name: str, arguments: dict | None) -> list[types.TextContent | types.ImageContent | types.EmbeddedResource]:
        results = await execute_tool(ctx, name, arguments or {})
        contents: list[types.TextContent | types.ImageContent | types.EmbeddedResource] = []
        for item in results:
            if item.get("type") == "image":
                contents.append(types.ImageContent(
                    type="image",
                    data=item["data"],
                    mimeType=item.get("mimeType", "image/png"),
                ))
            else:
                contents.append(types.TextContent(
                    type="text",
                    text=item.get("text", ""),
                ))
        return contents

    logger.info("Starting MCP server: %s v%s (profile=%s)", SERVER_NAME, SERVER_VERSION, profile_dir)

    try:
        async with stdio_server() as (read_stream, write_stream):
            await server.run(
                read_stream,
                write_stream,
                server.create_initialization_options(),
            )
    finally:
        await ctx.close()


def run_mcp_server(
    profile_name: str = "mcp",
    data_dir: Path | None = None,
    headed: bool = True,
) -> None:
    """Synchronous entry point for the MCP server.

    Parameters
    ----------
    profile_name:
        Name of the browser profile directory (isolated from the
        ``run`` sub-command to avoid Playwright lock conflicts).
    data_dir:
        Base data directory.  Defaults to the standard Worker data dir.
    headed:
        Show the browser window.  Defaults to ``True`` so the user can
        observe and intervene when needed.
    """
    from qiyuan_worker.config import default_data_dir

    resolved_data_dir = data_dir or default_data_dir()
    profile_dir = resolved_data_dir / "secrets" / "browser-profiles" / profile_name
    downloads_dir = resolved_data_dir / "downloads"

    profile_dir.mkdir(parents=True, exist_ok=True)
    downloads_dir.mkdir(parents=True, exist_ok=True)

    asyncio.run(_run_server(profile_dir, downloads_dir, headed))
