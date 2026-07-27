import { describe, expect, it } from "vitest";
import { compactID, formatJSON, statusLabel, statusTone } from "./viewModel";

describe("dashboard view model", () => {
  it("formats statuses for labels and tones", () => {
    expect(statusLabel("waiting_for_human")).toBe("waiting for human");
    expect(statusTone("promoted")).toBe("tone-good");
    expect(statusTone("rolled_back")).toBe("tone-bad");
  });

  it("keeps JSON readable", () => {
    expect(formatJSON({ b: 2, a: 1 })).toContain('"b": 2');
  });

  it("compacts long identifiers", () => {
    expect(compactID("1234567890abcdef")).toBe("12345678...cdef");
  });
});
