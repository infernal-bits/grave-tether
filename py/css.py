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
