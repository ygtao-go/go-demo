// Mock LLM Chat Completions server for integration testing.
// Returns a deterministic response that echoes the user prompt wrapped in markdown.
// Usage: node mock-llm.js
const http = require('http')

const server = http.createServer((req, res) => {
  let body = ''
  req.on('data', (chunk) => {
    body += chunk
  })
  req.on('end', () => {
    res.writeHead(200, { 'Content-Type': 'application/json' })
    try {
      const parsed = JSON.parse(body || '{}')
      const messages = Array.isArray(parsed.messages) ? parsed.messages : []
      const userMsg = messages.find((m) => m && m.role === 'user')
      const prompt = userMsg && typeof userMsg.content === 'string' ? userMsg.content : ''
      const content =
        '# Mock LLM 响应\n\n' +
        '```go\npackage main\n\nfunc main() {\n\tprintln("hello from mock llm")\n}\n```\n\n' +
        '## 收到的请求\n\n' +
        '- model: ' + String(parsed.model || '') + '\n' +
        '- prompt: ' + prompt + '\n'
      res.end(JSON.stringify({
        choices: [{ message: { role: 'assistant', content } }],
      }))
    } catch (e) {
      res.end(JSON.stringify({ error: e.message }))
    }
  })
})

server.listen(9100, '127.0.0.1', () => {
  console.log('mock-llm listening on 127.0.0.1:9100')
})
