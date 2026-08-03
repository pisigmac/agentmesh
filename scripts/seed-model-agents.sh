#!/usr/bin/env bash
# Register placeholder agents for model_* capabilities (optional mesh route metadata for chat).
# Capabilities align with deploy/models.yaml (KubeMind gateway providers).
set -euo pipefail
source "$(dirname "$0")/common.sh"

ORG="${ORG:-default}"
port=9910

put_agent() {
  local id="$1" cap="$2"
  curl -sf -X POST \
    -H "Authorization: Bearer ${AGENTMESH_DEV_API_KEY}" \
    -H "X-Org-ID: ${ORG}" \
    -H "Content-Type: application/json" \
    -d "{\"agent_id\":\"${id}\",\"agent_type\":\"custom\",\"endpoint_url\":\"http://127.0.0.1:${port}\",\"capabilities\":[\"${cap}\"],\"health\":{\"agent_id\":\"${id}\",\"healthy\":true},\"metadata\":{\"persistent\":\"true\",\"role\":\"model_gateway\"}}" \
    "${CONTROL_PLANE_URL}/v1/agents/register" >/dev/null
  echo "registered ${id} (${cap}) port=${port}"
  port=$((port + 1))
}

put_agent "model-ollama-gw" "model_ollama"
put_agent "model-vllm-gw" "model_vllm"
put_agent "model-deepseek-local-gw" "model_deepseek_local"
put_agent "model-groq-gw" "model_groq"
put_agent "model-google-gw" "model_google"
put_agent "model-openrouter-gw" "model_openrouter"
put_agent "model-openai-gw" "model_openai"
put_agent "model-anthropic-gw" "model_anthropic"
put_agent "model-custom-gw" "model_custom"

echo "Done. Save provider keys under Console → Models & keys (set base_url for local providers)."
