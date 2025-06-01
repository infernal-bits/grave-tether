import asyncio
import subprocess
import re
from textual.app import App, ComposeResult
from textual.widgets import Header, Footer, Static, Button, RichLog
from textual.containers import Container
from textual.reactive import reactive

# --- Configuration ---
ADB_PATH = "adb"  # Assumes 'adb' is in your PATH. Specify full path if not, e.g., "/usr/bin/adb"
# This variable needs to be handled carefully because it's modified by check_adb_connection
# If set here, it will be the preferred serial. If empty, check_adb_connection will try to auto-detect.
# It MUST be declared outside the class, and modified using 'nonlocal' in methods.
DEVICE_SERIAL = ""
# --- End Configuration ---

class AdbTetherApp(App):
    """A Textual app for ADB USB Tethering."""

    CSS = """
    #status-container {
        border: thick $primary;
        padding: 1 2;
        margin: 1 2;
        height: 8;
    }
    #status-title {
        text-align: center;
        text-style: bold;
        color: $accent;
    }
    #message-log {
        border: thick $secondary;
        padding: 0 1;
        margin: 1 2;
        height: 6;
        overflow-y: scroll;
    }
    #message-log-title {
        text-align: center;
        text-style: bold;
        color: $accent-lighten-1;
    }
    #buttons {
        layout: horizontal;
        align: center middle;
        height: 5;
        margin: 1 2;
        /* gap: 2;  <-- This line is commented out for wider compatibility */
    }
    Button {
        min-width: 18;
        background: $panel;
        color: $text;
        border: solid $primary;
    }
    Button:hover {
        background: $primary-darken-1;
    }
    Button.-active {
        background: $success;
        color: black;
    }
    .status-line {
        color: $text;
    }
    .status-ok {
        color: green;
        text-style: bold;
    }
    .status-warn {
        color: yellow;
        text-style: bold;
    }
    .status-error {
        color: red;
        text-style: bold;
    }
    """

    BINDINGS = [
        ("q", "quit", "Quit"),
        ("r", "refresh_status", "Refresh Status"),
    ]

    # Reactive variables to update UI elements
    adb_status = reactive("Unknown", layout=False)
    device_info = reactive("N/A", layout=False)
    linux_iface_status = reactive("Unknown", layout=False)
    adb_ready = reactive(False, layout=False)

    def compose(self) -> ComposeResult:
        yield Header()
        with Container(id="status-container"):
            yield Static("[#9B59B6]SYSTEM STATUS[/]", id="status-title") # Removed class="status-line" as it's a title
            yield Static("ADB Connection: [bold]Unknown[/]", id="adb-conn-status", classes="status-line")
            yield Static("Detected Device: [bold]N/A[/]", id="device-info", classes="status-line")
            yield Static("Linux Interface: [bold]Unknown[/]", id="linux-iface-status", classes="status-line")
        with Container(id="message-log"):
            yield Static("[#6A5ACD]MESSAGES / OUTPUT[/]", id="message-log-title")
            yield RichLog(auto_scroll=True, markup=True, highlight=True)
        with Container(id="buttons"):
            yield Button("Enable Tethering", id="enable-btn", variant="success")
            yield Button("Disable Tethering", id="disable-btn", variant="error")
            yield Button("Refresh Status", id="refresh-btn", variant="primary")
        yield Footer()

    async def on_mount(self) -> None:
        # Refresh status every 5 seconds, starting immediately
        self.set_interval(5, self.action_refresh_status)
        await self.action_refresh_status() # Initial status check

    async def action_refresh_status(self) -> None:
        self.query_one(RichLog).write("[cyan]Refreshing status...[/cyan]")
        await self.check_adb_connection()
        await self.get_linux_interface_status()
        self.query_one(RichLog).write("[green]Status refreshed.[/green]")

    def watch_adb_status(self, status: str) -> None:
        self.query_one("#adb-conn-status", Static).update(f"ADB Connection: {status}")

    def watch_device_info(self, info: str) -> None:
        self.query_one("#device-info", Static).update(f"Detected Device: {info}")

    def watch_linux_iface_status(self, status: str) -> None:
        self.query_one("#linux-iface-status", Static).update(f"Linux Interface: {status}")

    async def run_adb_command(self, description: str, command_args: list[str]) -> bool:
        # Indentation level 1: Method body
        full_command = [ADB_PATH]
        if DEVICE_SERIAL: # Indentation level 2: if block
            full_command.extend(["-s", DEVICE_SERIAL])
        full_command.extend(command_args)

        self.query_one(RichLog).write(f"[blue]Executing: {description}...[/blue]")
        self.query_one(RichLog).write(f"[yellow]  `{' '.join(full_command)}`[/yellow]")

        try: # Indentation level 2: start of try block
            process = await asyncio.create_subprocess_exec(
                *full_command,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            stdout, stderr = await process.communicate()

            if process.returncode == 0: # Indentation level 3: if block
                self.query_one(RichLog).write(f"[green]SUCCESS: {description}[/green]")
                if stdout: # Indentation level 4: inner if block
                    self.query_one(RichLog).write(f"[dim]Output:[/dim] {stdout.decode().strip()}")
                return True
            else: # Indentation level 3: else block
                self.query_one(RichLog).write(f"[red]FAILED: {description}[/red]")
                if stdout: # Indentation level 4: inner if block
                    self.query_one(RichLog).write(f"[dim]Stdout:[/dim] {stdout.decode().strip()}")
                if stderr: # Indentation level 4: inner if block
                    self.query_one(RichLog).write(f"[dim]Stderr:[/dim] {stderr.decode().strip()}")
                return False
        except FileNotFoundError: # Indentation level 2: except block (MUST match 'try' indentation)
            self.query_one(RichLog).write(f"[red]ERROR: ADB command not found. Is '{ADB_PATH}' in your PATH?[/red]")
            return False
        except Exception as e: # Indentation level 2: except block (MUST match 'try' indentation)
            self.query_one(RichLog).write(f"[red]ERROR executing command: {e}[/red]")
            return False

    async def check_adb_connection(self) -> None:
        # Indentation level 1: Method body
        global DEVICE_SERIAL # This needs to be at the start of the method body

        try: # Indentation level 2: start of try block
            proc = await asyncio.create_subprocess_exec(
                ADB_PATH, "devices",
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            stdout, _ = await proc.communicate()
            output = stdout.decode().strip()

            # THIS IS LIKELY WHERE YOUR INDENTATION IS WRONG (should be indentation level 3)
            devices = [line.split('\t')[0] for line in output.splitlines() if 'device' in line and 'List of devices attached' not in line]

            if not devices: # Indentation level 3: if block
                self.adb_status = "[red]Disconnected[/red]"
                self.device_info = "N/A"
                self.adb_ready = False
                self.query_one(RichLog).write("[red]No Android device detected. Connect phone & enable USB debugging.[/red]")
            elif len(devices) > 1 and not DEVICE_SERIAL: # Indentation level 3: elif block
                self.adb_status = "[yellow]Multiple Devices[/yellow]"
                self.device_info = f"Please specify serial (found: {', '.join(devices)})"
                self.adb_ready = False
                self.query_one(RichLog).write(f"[yellow]Multiple devices found. Please set `DEVICE_SERIAL` in the script to select one: {', '.join(devices)}[/yellow]")
            else: # Indentation level 3: else block
                if not DEVICE_SERIAL: # Indentation level 4: if block
                    DEVICE_SERIAL = devices[0] # Set the global variable
                elif DEVICE_SERIAL not in devices: # Indentation level 4: elif block
                    self.adb_status = "[red]Specified Device Not Found[/red]"
                    self.device_info = DEVICE_SERIAL
                    self.adb_ready = False
                    self.query_one(RichLog).write(f"[red]Specified device ({DEVICE_SERIAL}) not found among connected devices. Detected: {', '.join(devices)}.[/red]")
                    return # Exit early if specified device isn't found

                # Final check for authorization
                test_proc = await asyncio.create_subprocess_exec(
                    ADB_PATH, "-s", DEVICE_SERIAL, "shell", "echo", "test",
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE
                )
                _, stderr = await test_proc.communicate()
                if b"device unauthorized" in stderr: # Indentation level 4: if block
                    self.adb_status = "[red]Auth Failed[/red]"
                    self.device_info = DEVICE_SERIAL
                    self.adb_ready = False
                    self.query_one(RichLog).write(f"[red]Failed to communicate with {DEVICE_SERIAL}. Ensure authorization (prompt on phone).[/red]")
                else: # Indentation level 4: else block
                    self.adb_status = "[green]Connected[/green]"
                    self.device_info = DEVICE_SERIAL
                    self.adb_ready = True

        except FileNotFoundError: # Indentation level 2: except block (MUST match 'try' indentation)
            self.adb_status = "[red]ERROR: ADB command not found[/red]"
            self.device_info = "N/A"
            self.adb_ready = False
            self.query_one(RichLog).write(f"[red]ERROR: '{ADB_PATH}' command not found. Please install `android-tools` package.[/red]")
        except Exception as e: # Indentation level 2: except block (MUST match 'try' indentation)
            self.adb_status = "[red]Connection Error[/red]"
            self.device_info = "N/A"
            self.adb_ready = False
            self.query_one(RichLog).write(f"[red]An error occurred during ADB check: {e}[/red]")

    async def get_linux_interface_status(self) -> None:
        # Indentation level 1: Method body
        try: # Indentation level 2: start of try block
            proc = await asyncio.create_subprocess_exec(
                "ip", "link", "show",
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
            stdout, _ = await proc.communicate()
            output = stdout.decode().strip()

            # THIS LINE should be indentation level 3
            match = re.search(r'\d+: (usb\d+|rndis\d+|enp\S+u\d+): <.*BROADCAST,MULTICAST,UP.*>', output)

            if match: # Indentation level 3: if block
                interface_name = match.group(1)
                ip_proc = await asyncio.create_subprocess_exec(
                    "ip", "-4", "addr", "show", "dev", interface_name,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE
                )
                ip_stdout, _ = await ip_proc.communicate()
                ip_output = ip_stdout.decode().strip()

                ip_match = re.search(r'inet (\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})', ip_output)
                if ip_match: # Indentation level 4: if block
                    self.linux_iface_status = f"[green]{interface_name} (IP: {ip_match.group(1)})[/green]"
                    self.query_one(RichLog).write(f"[green]Linux interface '{interface_name}' detected with IP: {ip_match.group(1)}.[/green]")
                else: # Indentation level 4: else block
                    self.linux_iface_status = f"[yellow]{interface_name} (No IP)[/yellow]"
                    self.query_one(RichLog).write(f"[yellow]Linux interface '{interface_name}' detected, but no IP. Needs DHCP config.[/yellow]")
            else: # Indentation level 3: else block
                self.linux_iface_status = "[red]Not Detected[/red]"
                self.query_one(RichLog).write("[red]No active USB-related network interface found on Linux.[/red]")

        except FileNotFoundError: # Indentation level 2: except block (MUST match 'try' indentation)
            self.query_one(RichLog).write(f"[red]ERROR: 'ip' command not found. Is 'iproute2' installed?[/red]")
            self.linux_iface_status = "[red]ERROR: 'ip' missing[/red]"
        except Exception as e: # Indentation level 2: except block (MUST match 'try' indentation)
            self.query_one(RichLog).write(f"[red]An error occurred during Linux interface check: {e}[/red]")
            self.linux_iface_status = "[red]Error Checking[/red]"


    async def on_button_pressed(self, event: Button.Pressed) -> None:
        # IMPORTANT: For commands that modify device state, ensure DEVICE_SERIAL is set
        # and adb_ready is True before proceeding.
        if not DEVICE_SERIAL or not self.adb_ready:
            self.query_one(RichLog).write("[red]ADB not connected, authorized, or device not selected. Cannot perform action.[/red]")
            return

        if event.button.id == "enable-btn":
            self.query_one(RichLog).write("[bold green]Attempting to ENABLE tethering...[/bold green]")
            # Attempt root (often fails on stock ROMs, but harmless to try)
            await self.run_adb_command("Attempting ADB root", ["root"])
            # Core commands
            success = await self.run_adb_command("Set tether_dun_required to 0", ["shell", "settings", "put", "global", "tether_dun_required", "0"])
            if success: # Only proceed if the previous command was successful, for sequential logic
                success = await self.run_adb_command("Set USB functions to RNDIS", ["shell", "svc", "usb", "setFunctions", "rndis"])
            if success:
                await self.run_adb_command("Disable tether_offload", ["shell", "settings", "put", "global", "tether_offload_disabled", "1"])
            self.query_one(RichLog).write("[bold green]Tethering commands sent. Refreshing status.[/bold green]")
            await self.action_refresh_status()
            self.query_one(RichLog).write("[dim]Reminder: You might need to manually configure DHCP on the new Linux interface.[/dim]")

        elif event.button.id == "disable-btn":
            self.query_one(RichLog).write("[bold yellow]Attempting to DISABLE tethering...[/bold yellow]")
            # Core commands (reverse order is good practice for cleanup)
            success = await self.run_adb_command("Restore tether_offload_disabled to 0", ["shell", "settings", "put", "global", "tether_offload_disabled", "0"])
            if success:
                success = await self.run_adb_command("Restore tether_dun_required to 1", ["shell", "settings", "put", "global", "tether_dun_required", "1"])
            if success:
                await self.run_adb_command("Restore default USB functions (MTP, ADB)", ["shell", "svc", "usb", "setFunctions", "mtp,adb"])
            self.query_one(RichLog).write("[bold yellow]Tethering deactivation commands sent. Refreshing status.[/bold yellow]")
            await self.action_refresh_status()

        elif event.button.id == "refresh-btn":
            await self.action_refresh_status()

if __name__ == "__main__":
    # Ensure Textual is updated before running.
    # If `pip install textual --upgrade` didn't work, ensure your pip is
    # associated with Python 3.13.3 (e.g., `python3.13 -m pip install textual --upgrade`).

    app = AdbTetherApp()
    app.run()
