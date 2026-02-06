#!/bin/bash
# Validate all CLI agents and models work correctly
# Uses minimal prompts to save tokens

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

PROMPT="say ok"
TIMEOUT=60

passed=0
failed=0
skipped=0

test_model() {
    local agent=$1
    local model=$2
    local extra_args=$3

    printf "  %-30s " "$model"

    case $agent in
        claude)
            result=$(timeout $TIMEOUT claude -p --model "$model" --max-turns 1 "$PROMPT" 2>&1) && status=$? || status=$?
            ;;
        gemini)
            result=$(timeout $TIMEOUT gemini -m "$model" -p "$PROMPT" 2>&1) && status=$? || status=$?
            ;;
        codex)
            # Some models need lower reasoning effort (config has xhigh default)
            case "$model" in
                gpt-5.3-codex|gpt-5.2-codex|gpt-5.2|gpt-5.1-codex-max)
                    result=$(timeout $TIMEOUT codex exec -m "$model" $extra_args "$PROMPT" 2>&1) && status=$? || status=$?
                    ;;
                gpt-5.1-codex-mini)
                    result=$(timeout $TIMEOUT codex exec -m "$model" -c 'model_reasoning_effort="medium"' $extra_args "$PROMPT" 2>&1) && status=$? || status=$?
                    ;;
                *)
                    result=$(timeout $TIMEOUT codex exec -m "$model" -c 'model_reasoning_effort="high"' $extra_args "$PROMPT" 2>&1) && status=$? || status=$?
                    ;;
            esac
            ;;
        copilot)
            result=$(timeout $TIMEOUT copilot -p "$PROMPT" --model "$model" --allow-all-tools -s 2>&1) && status=$? || status=$?
            ;;
    esac

    if [ $status -eq 0 ]; then
        echo -e "${GREEN}OK${NC}"
        ((passed++))
    elif [ $status -eq 124 ]; then
        echo -e "${YELLOW}TIMEOUT${NC}"
        ((skipped++))
    else
        echo -e "${RED}FAIL${NC}"
        ((failed++))
        # Show first line of error (truncated)
        echo "    $(echo "$result" | grep -i "error\|fail\|not supported" | head -1 | cut -c1-70)"
    fi
}

echo "=========================================="
echo "Model Validation Test"
echo "Prompt: \"$PROMPT\" (minimal)"
echo "=========================================="
echo ""

# CLAUDE - Test all aliases
echo -e "${BLUE}CLAUDE${NC}"
test_model claude "opus"
test_model claude "sonnet"
test_model claude "haiku"
echo ""

# GEMINI - Test stable and preview models
echo -e "${BLUE}GEMINI${NC}"
test_model gemini "gemini-2.5-flash"
test_model gemini "gemini-2.5-pro"
test_model gemini "gemini-3-pro-preview"
test_model gemini "gemini-3-flash-preview"
echo ""

# CODEX - Test models that work with ChatGPT account
echo -e "${BLUE}CODEX${NC}"
test_model codex "gpt-5.3-codex"
test_model codex "gpt-5.2-codex"
test_model codex "gpt-5.2"
test_model codex "gpt-5.1-codex-max"
test_model codex "gpt-5.1-codex"
test_model codex "gpt-5.1-codex-mini"
test_model codex "gpt-5.1"
test_model codex "gpt-5-codex"
test_model codex "gpt-5"
echo ""

# COPILOT - Test models from copilot --help
echo -e "${BLUE}COPILOT${NC}"
test_model copilot "claude-sonnet-4.5"
test_model copilot "claude-opus-4.6"
test_model copilot "claude-haiku-4.5"
test_model copilot "gpt-5.2-codex"
test_model copilot "gpt-5.1-codex-max"
test_model copilot "gpt-5"
test_model copilot "gemini-3-pro-preview"
echo ""

echo "=========================================="
echo -e "Results: ${GREEN}$passed passed${NC}, ${RED}$failed failed${NC}, ${YELLOW}$skipped timeout${NC}"
echo "=========================================="

if [ $failed -gt 0 ]; then
    exit 1
fi
exit 0
