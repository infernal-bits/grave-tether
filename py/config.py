# --- Configuration ---
ADB_PATH = "adb"  # Assumes 'adb' is in your PATH. Specify full path if not, e.g., "/usr/bin/adb"
# This variable needs to be handled carefully because it's modified by check_adb_connection
# If set here, it will be the preferred serial. If empty, check_adb_connection will try to auto-detect.
# It MUST be declared outside the class, and modified using 'nonlocal' in methods.
DEVICE_SERIAL = ""
# --- End Configuration ---


