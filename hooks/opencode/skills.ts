// Downshift OpenCode bridge module (wire v1). Written by `downshift setup`; edit nothing here.
//
// Skill adoption forwarding (counts only; Go normalizes and enforces privacy).

import { HARNESS, WIRE_VERSION, answer, opaque } from "./wire.ts"

const PROMPT_SKILL_RE = /(?:^|[\s(\[{"'])[/$](?:(?:downshift:)?ponytail|(?:downshift:)?downshift-[a-z0-9-]+|superpowers:[a-z0-9-]+)/i

export function observeSkillTool(bin: string, version: string, event: any): void {
  const tool = typeof event?.tool === "string" ? event.tool : ""
  if (!tool) return
  const sessionID = typeof event?.sessionID === "string" ? event.sessionID : ""
  if (!sessionID) return
  const input = event?.input && typeof event.input === "object" ? event.input : {}
  let skill = ""
  let path = ""
  if (tool.toLowerCase() === "skill") {
    skill = typeof input.name === "string" ? input.name : (typeof input.skill === "string" ? input.skill : "")
    if (!skill) return
  } else if (tool === "read" || tool === "read_file") {
    path = typeof input.filePath === "string" ? input.filePath : (typeof input.path === "string" ? input.path : (typeof input.file_path === "string" ? input.file_path : ""))
    if (!/(?:^|[\\/])SKILL\.md$/i.test(path)) return
  } else {
    return
  }
  answer(bin, [HARNESS], {
    v: WIRE_VERSION,
    kind: "skill",
    event: "skill_used",
    harness: { id: HARNESS, version },
    correlation: { child: opaque(sessionID), parent: "" },
    tool,
    skill,
    path,
    tsUtc: new Date().toISOString(),
  })
}

export function observeSkillPrompt(bin: string, version: string, sessionID: string, text: string): void {
  if (!sessionID || typeof text !== "string" || !PROMPT_SKILL_RE.test(text)) return
  answer(bin, [HARNESS], {
    v: WIRE_VERSION,
    kind: "skill",
    event: "skill_used",
    harness: { id: HARNESS, version },
    correlation: { child: opaque(sessionID), parent: "" },
    taskText: text,
    tsUtc: new Date().toISOString(),
  })
}
