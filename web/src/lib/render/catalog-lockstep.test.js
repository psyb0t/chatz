import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { GENUI_COMPONENT_NAMES } from "$lib/common/render";
import { genUICatalog } from "./catalog";
import { registry } from "./registry";

describe("GenUI catalog lockstep", () => {
  it("matches constants, catalog, registry, generator, and generated prompt", () => {
    const generatorPath = `${process.cwd()}/scripts/gen-ui-instructions.mjs`;
    const generatedPromptPath = `${process.cwd()}/../internal/pkg/core/chats/prompts/genui_instructions.gen.txt`;
    const generator = spawnSync(
      process.execPath,
      [generatorPath, "--list-components"],
      { encoding: "utf8" },
    );

    expect(generator.status, generator.stderr).toBe(0);
    const generatorNames = JSON.parse(generator.stdout);
    const prompt = readFileSync(generatedPromptPath, "utf8");
    const promptNames = [
      ...prompt.matchAll(/^- ([A-Za-z][A-Za-z0-9]*): \{/gm),
    ].map((match) => match[1]);
    const expectedNames = [...GENUI_COMPONENT_NAMES];

    expect(genUICatalog.componentNames).toEqual(expectedNames);
    expect(Object.keys(registry)).toEqual(expectedNames);
    expect(generatorNames).toEqual(expectedNames);
    expect(promptNames).toEqual(expectedNames);
    expect(prompt).toContain(`AVAILABLE COMPONENTS (${expectedNames.length}):`);
  });
});
