#!/bin/sh
cmd=$(jq -r '.tool_input.command // empty')
[ -z "$cmd" ] && exit 0

reason=""
if printf '%s' "$cmd" | grep -Eq '(^|[;&|(` /])mdv([[:space:]]|$)' && printf '%s' "$cmd" | grep -Eq -- '--yes'; then
  reason="mdv write (--yes): approve only if you already agreed to this exact action in chat."
elif printf '%s' "$cmd" | grep -Eq '(^|[;&|(` /])ntn([[:space:]]|$)'; then
  reason="Direct ntn call: agents should use mdv for Notion, not ntn."
elif printf '%s' "$cmd" | grep -Eq '(^|[;&|(` /])gh[[:space:]]+gist([[:space:]]|$)'; then
  reason="Direct gh gist call: agents should use mdv share, not gh gist."
fi

[ -z "$reason" ] && exit 0
jq -n --arg r "$reason" '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "ask", permissionDecisionReason: $r}}'
