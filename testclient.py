import sys
import json

from websocket import create_connection


ws = create_connection(sys.argv[1])
msg = json.loads(ws.recv())
print msg
assert msg['event'] == 'pusher:connection_established'

ws.send('{"event": "pusher:subscribe", "data": {"channel": "public-test"}}')
# TODO: response of subscribe

ws.send('{"event": "client-hello", "channel": "public-test", "data": {"hello": "world"}}')
msg = json.loads(ws.recv())
print msg
assert msg["event"] == "client-hello"
assert msg["data"] == {"hello": "world"}
