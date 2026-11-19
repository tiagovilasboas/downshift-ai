// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// Child session tracking and per-step token accumulation.

import { type ModelRef } from "./models.ts"
import { count } from "./wire.ts"

export type Usage = {
  input: number
  output: number
  reasoning: number
  cache: { read: number; write: number }
}

export type Child = {
  sessionID: string
  child: string
  parent: string
  agent: string
  directory: string
  requested: ModelRef
  target: ModelRef | null
  switchResult: "ok" | "error" | ""
  contextModel: string
  fallback: boolean
  usage: Usage
  steps: Set<string>
  finished: boolean
}

export function recordStepUsage(child: Child, data: any): void {
  const messageID = typeof data?.assistantMessageID === "string" ? data.assistantMessageID : ""
  if (messageID && child.steps.has(messageID)) return
  if (messageID) child.steps.add(messageID)
  const tokens = data?.tokens && typeof data.tokens === "object" ? data.tokens : {}
  const cache = tokens.cache && typeof tokens.cache === "object" ? tokens.cache : {}
  child.usage.input += count(tokens.input)
  child.usage.output += count(tokens.output)
  child.usage.reasoning += count(tokens.reasoning)
  child.usage.cache.read += count(cache.read)
  child.usage.cache.write += count(cache.write)
}
