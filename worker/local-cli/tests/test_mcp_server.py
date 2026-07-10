"""Unit tests for MCP server module.

Tests verify server construction, tool registration, and entry point
wiring without starting the actual stdio transport.
"""

from __future__ import annotations

import unittest
from pathlib import Path
from unittest.mock import patch, MagicMock

from qiyuan_worker.mcp.server import SERVER_NAME, SERVER_VERSION, run_mcp_server
from qiyuan_worker.mcp.tools import TOOLS


class ServerMetadataTest(unittest.TestCase):
    def test_server_name(self) -> None:
        self.assertEqual(SERVER_NAME, "qiyuan-browser")

    def test_server_version(self) -> None:
        self.assertRegex(SERVER_VERSION, r"^\d+\.\d+\.\d+$")


class ToolRegistrationTest(unittest.TestCase):
    def test_all_tools_registered(self) -> None:
        """Every tool in TOOLS should have a handler in _HANDLERS."""
        from qiyuan_worker.mcp.tools import _HANDLERS
        for tool in TOOLS:
            self.assertIn(tool.name, _HANDLERS, f"Tool {tool.name} has no handler")


class RunMcpServerEntryTest(unittest.TestCase):
    """Test the synchronous entry point wiring."""

    @patch("qiyuan_worker.mcp.server.asyncio")
    @patch("qiyuan_worker.mcp.server.BrowserRuntime")
    def test_run_mcp_server_creates_dirs(self, mock_runtime_cls, mock_asyncio) -> None:
        """run_mcp_server should create profile and download directories."""
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            data_dir = Path(tmp)
            # asyncio.run will be called; we just prevent it from blocking.
            mock_asyncio.run = MagicMock()

            run_mcp_server(profile_name="test-profile", data_dir=data_dir, headed=False)

            profile_dir = data_dir / "secrets" / "browser-profiles" / "test-profile"
            downloads_dir = data_dir / "downloads"
            self.assertTrue(profile_dir.is_dir())
            self.assertTrue(downloads_dir.is_dir())
            mock_asyncio.run.assert_called_once()


class CLIIntegrationTest(unittest.TestCase):
    """Test that the mcp subcommand is registered in the CLI parser."""

    def test_mcp_subcommand_exists(self) -> None:
        from qiyuan_worker.cli import build_parser

        parser = build_parser()
        # parse_args should succeed for 'mcp'
        args = parser.parse_args(["mcp"])
        self.assertTrue(hasattr(args, "func"))
        self.assertEqual(args.profile, "mcp")
        self.assertTrue(args.headed)
        self.assertFalse(args.headless)

    def test_mcp_headless_flag(self) -> None:
        from qiyuan_worker.cli import build_parser

        parser = build_parser()
        args = parser.parse_args(["mcp", "--headless", "--profile", "ci"])
        self.assertTrue(args.headless)
        self.assertEqual(args.profile, "ci")


if __name__ == "__main__":
    unittest.main()
