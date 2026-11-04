import React, { useState } from 'react';
import { Copy, Check } from 'lucide-react';

interface CodeGeneratorProps {
  apiKey?: string;
  endpoint?: string;
}

type Language = 'curl' | 'typescript' | 'python' | 'go' | 'rust';
type Mode = 'proxy' | 'verify';

export const CodeGenerator: React.FC<CodeGeneratorProps> = ({
  apiKey = 'sk_live_ws_default_sampleKey123',
  endpoint = 'http://localhost:8000',
}) => {
  const [lang, setLang] = useState<Language>('curl');
  const [mode, setMode] = useState<Mode>('proxy');
  const [copied, setCopied] = useState(false);

  const getSnippet = (): string => {
    if (mode === 'proxy') {
      switch (lang) {
        case 'curl':
          return `curl -X GET "${endpoint}/api/v1/users" \\
  -H "Authorization: Bearer ${apiKey}"`;

        case 'typescript':
          return `// TypeScript / Node.js 18+ (Fetch API)
const response = await fetch("${endpoint}/api/v1/users", {
  method: "GET",
  headers: {
    "Authorization": "Bearer ${apiKey}",
    "Content-Type": "application/json"
  }
});

// Check IETF rate limit headers
const remaining = response.headers.get("RateLimit-Remaining");
const reset = response.headers.get("RateLimit-Reset");

const data = await response.json();
console.log(data);`;

        case 'python':
          return `# Python 3 (httpx or requests)
import httpx

headers = {
    "Authorization": "Bearer ${apiKey}",
    "Content-Type": "application/json"
}

with httpx.Client() as client:
    response = client.get("${endpoint}/api/v1/users", headers=headers)
    print("Status:", response.status_code)
    print("RateLimit-Remaining:", response.headers.get("RateLimit-Remaining"))
    print("Body:", response.json())`;

        case 'go':
          return `// Go 1.23+
package main

import (
	"fmt"
	"io"
	"net/http"
)

func main() {
	req, _ := http.NewRequest("GET", "${endpoint}/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer ${apiKey}")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("Status:", resp.Status)
	fmt.Println("Remaining:", resp.Header.Get("RateLimit-Remaining"))
	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))
}`;

        case 'rust':
          return `// Rust (reqwest async)
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let res = client.get("${endpoint}/api/v1/users")
        .header("Authorization", "Bearer ${apiKey}")
        .send()
        .await?;

    println!("Status: {}", res.status());
    let body = res.text().await?;
    println!("Response: {}", body);
    Ok(())
}`;
      }
    } else {
      // Direct Verification Endpoint
      switch (lang) {
        case 'curl':
          return `curl -X POST "${endpoint}/v1/keys.verifyKey" \\
  -H "Content-Type: application/json" \\
  -d '{"key": "${apiKey}", "requiredScopes": ["read:users"]}'`;

        case 'typescript':
          return `// Standalone verification in microservices
const res = await fetch("${endpoint}/v1/keys.verifyKey", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    key: "${apiKey}",
    requiredScopes: ["read:users"]
  })
});

const verification = await res.json();
if (!verification.valid) {
  throw new Error(\`Unauthorized: \${verification.code}\`);
}
console.log("Verified Key ID:", verification.keyId);`;

        case 'python':
          return `# Verify key out-of-band in Python
import httpx

payload = {
    "key": "${apiKey}",
    "requiredScopes": ["read:users"]
}

res = httpx.post("${endpoint}/v1/keys.verifyKey", json=payload)
data = res.json()
if not data.get("valid"):
    print("Invalid key:", data.get("code"))
else:
    print("Valid key for owner:", data.get("ownerId"))`;

        case 'go':
          return `// Standalone verification in Go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

func main() {
	payload := map[string]interface{}{
		"key": "${apiKey}",
		"requiredScopes": []string{"read:users"},
	}
	body, _ := json.Marshal(payload)

	resp, _ := http.Post("${endpoint}/v1/keys.verifyKey", "application/json", bytes.NewReader(body))
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	fmt.Printf("Validation Result: %+v\\n", result)
}`;

        case 'rust':
          return `// Rust key verification
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let client = reqwest::Client::new();
    let res = client.post("${endpoint}/v1/keys.verifyKey")
        .json(&serde_json::json!({
            "key": "${apiKey}",
            "requiredScopes": ["read:users"]
        }))
        .send()
        .await?;

    let json: serde_json::Value = res.json().await?;
    println!("Valid: {}", json["valid"]);
    Ok(())
}`;
      }
    }
  };

  const handleCopy = () => {
    navigator.clipboard.writeText(getSnippet());
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="bg-surface border border-border rounded-xl overflow-hidden">
      {/* Top Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between border-b border-border bg-background/70 px-4 py-2.5 gap-2">
        {/* Mode Selector */}
        <div className="flex bg-surface p-0.5 rounded-lg border border-border text-xs font-medium">
          <button
            onClick={() => setMode('proxy')}
            className={`px-3 py-1 rounded-md transition-colors ${
              mode === 'proxy'
                ? 'bg-primary-600 text-white font-semibold'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            Reverse Proxy
          </button>
          <button
            onClick={() => setMode('verify')}
            className={`px-3 py-1 rounded-md transition-colors ${
              mode === 'verify'
                ? 'bg-primary-600 text-white font-semibold'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            Direct Verification API
          </button>
        </div>

        {/* Language Tabs & Copy Button */}
        <div className="flex items-center gap-2">
          <div className="flex gap-1 text-xs">
            {(['curl', 'typescript', 'python', 'go', 'rust'] as const).map((l) => (
              <button
                key={l}
                onClick={() => setLang(l)}
                className={`px-2.5 py-1 rounded capitalize transition-colors font-mono text-[11px] ${
                  lang === l
                    ? 'bg-surface-hover text-white font-bold border border-border'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                {l === 'typescript' ? 'TS / JS' : l}
              </button>
            ))}
          </div>

          <button
            onClick={handleCopy}
            className="flex items-center gap-1.5 px-3 py-1 text-xs font-medium text-slate-200 bg-surface hover:bg-slate-800 border border-border rounded-lg transition-colors ml-2"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      </div>

      {/* Code Display */}
      <pre className="p-4 bg-background text-xs font-mono text-slate-300 overflow-x-auto leading-relaxed">
        {getSnippet()}
      </pre>
    </div>
  );
};
