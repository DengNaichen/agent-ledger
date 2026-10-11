"""Probe the installed Codex MCP client using only disposable fake credentials.

Requires Codex on PATH. No model calls, real UC tokens, or user config changes.
The fixture waits 36 seconds to cross its access token lifetime; each RPC wait
is bounded to 20 seconds. This checks client behavior, not the InfTask server.
"""
import tempfile, json, hashlib, os, subprocess, threading, time, queue
from pathlib import Path
from http.server import ThreadingHTTPServer, BaseHTTPRequestHandler
from urllib.parse import parse_qs, urlsplit
requests = []
stage = 0
refresh_number = 0

class Handler(BaseHTTPRequestHandler):

    def log_message(self, *args):
        pass

    def reply(self, status, data, headers={}):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        for (k, v) in headers.items():
            self.send_header(k, v)
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

    def do_GET(self):
        if 'oauth-protected-resource' in self.path:
            self.reply(200, {'resource': url, 'authorization_servers': [origin], 'bearer_methods_supported': ['header']})
        elif 'oauth-authorization-server' in self.path or 'openid-configuration' in self.path:
            self.reply(200, {'issuer': origin, 'authorization_endpoint': origin + '/authorize', 'token_endpoint': origin + '/token', 'response_types_supported': ['code'], 'grant_types_supported': ['authorization_code', 'refresh_token'], 'code_challenge_methods_supported': ['S256'], 'token_endpoint_auth_methods_supported': ['none']})
        elif self.path == '/w/default/mcp':
            self.reply(405, {})
        else:
            self.reply(404, {})

    def do_POST(self):
        global refresh_number
        body = self.rfile.read(int(self.headers.get('Content-Length', '0')))
        if self.path == '/token':
            form = parse_qs(body.decode())
            requests.append({'stage': stage, 'fields': sorted(form), 'grant_type': form.get('grant_type'), 'uses_latest_refresh': form.get('refresh_token') == ['test-refresh' if refresh_number == 0 else 'refresh-' + str(refresh_number)]})
            if stage == 0:
                self.reply(503, {'error': 'temporarily_unavailable'})
                return
            refresh_number += 1
            self.reply(200, {'access_token': 'access-' + str(refresh_number), 'refresh_token': 'refresh-' + str(refresh_number), 'token_type': 'Bearer', 'expires_in': 35})
            return
        data = json.loads(body)
        if 'id' not in data:
            self.send_response(202)
            self.end_headers()
            return
        method = data['method']
        if method == 'initialize':
            result = {'protocolVersion': data['params']['protocolVersion'], 'capabilities': {'tools': {}}, 'serverInfo': {'name': 'al39-fixture', 'version': '1'}}
        elif method == 'tools/list':
            result = {'tools': [{'name': 'list_teams', 'description': 'Fixture only', 'inputSchema': {'type': 'object', 'properties': {}}}]}
        else:
            result = {}
        self.reply(200, {'jsonrpc': '2.0', 'id': data['id'], 'result': result})
server = ThreadingHTTPServer(('127.0.0.1', 0), Handler)
origin = 'http://127.0.0.1:' + str(server.server_port)
url = origin + '/w/default/mcp'
threading.Thread(target=server.serve_forever, daemon=True).start()
with tempfile.TemporaryDirectory(prefix='al39-codex-') as tmp:
    root = Path(tmp)
    (root / 'config.toml').write_text('mcp_oauth_credentials_store = "file"\n[mcp_servers.probe]\nurl = "' + url + '"\n[mcp_servers.probe.oauth]\nclient_id = "inftask"\n')
    payload = json.dumps({'headers': {}, 'type': 'http', 'url': url}, separators=(',', ':'), sort_keys=True)
    key = 'probe|' + hashlib.sha256(payload.encode()).hexdigest()[:16]
    (root / '.credentials.json').write_text(json.dumps({key: {'server_name': 'probe', 'server_url': url, 'issuer': origin, 'client_id': 'inftask', 'access_token': 'test-expired', 'expires_at': int(time.time() * 1000) - 60000, 'refresh_token': 'test-refresh', 'scopes': []}}))
    env = os.environ.copy()
    env['CODEX_HOME'] = tmp
    for stage in range(3):
        p = subprocess.Popen(['codex', 'app-server'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, env=env, cwd=tmp)
        q = queue.Queue()
        threading.Thread(target=lambda : [q.put(line) for line in p.stdout], daemon=True).start()

        def send(d):
            p.stdin.write(json.dumps(d) + '\n')
            p.stdin.flush()
        send({'id': 1, 'method': 'initialize', 'params': {'clientInfo': {'name': 'al39-test', 'version': '1'}, 'capabilities': {'experimentalApi': True}}})
        deadline = time.monotonic() + 20
        d = {}
        try:
            while time.monotonic() < deadline:
                try:
                    d = json.loads(q.get(timeout=1))
                except queue.Empty:
                    continue
                if d.get('id') == 1:
                    send({'method': 'initialized', 'params': {}})
                    send({'id': 2, 'method': 'mcpServerStatus/list', 'params': {}})
                if d.get('id') == 2:
                    break
            assert d.get('id') == 2, 'Timed out waiting for mcpServerStatus/list'
            if stage > 0:
                servers = d.get('result', {}).get('data', [])
                assert any(('list_teams' in x.get('tools', {}) for x in servers)), 'MCP tool discovery did not recover'
            cached = json.loads((root / '.credentials.json').read_text())
            print(json.dumps({'stage': stage, 'response_received': d.get('id') == 2, 'credentials_retained': any((bool(x.get('refresh_token')) for x in cached.values())), 'refresh_requests': [x for x in requests if x['stage'] == stage]}))
        finally:
            p.terminate()
            p.wait(timeout=10)
        if stage == 1:
            time.sleep(36)
    server.shutdown()
    assert refresh_number >= 2, 'Did not complete two rotations'
    assert all((x['uses_latest_refresh'] for x in requests)), 'Stale refresh token reused'
