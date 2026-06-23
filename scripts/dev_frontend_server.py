#!/usr/bin/env python3
import http.server
import sys


class NoCacheHTTPRequestHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Cache-Control", "no-store, no-cache, must-revalidate")
        super().end_headers()


if __name__ == "__main__":
    port = int(sys.argv[1])
    root = sys.argv[2]
    http.server.ThreadingHTTPServer(
        ("", port),
        lambda *args, **kwargs: NoCacheHTTPRequestHandler(*args, directory=root, **kwargs),
    ).serve_forever()
