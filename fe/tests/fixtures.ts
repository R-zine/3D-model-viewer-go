import { Buffer } from "node:buffer";
import { createDemoModel } from "../src/lib/demoModel";

export function demoModelBuffer(): Buffer {
  return Buffer.from(createDemoModel());
}

export function jsonGLB(document: unknown, binary?: Uint8Array): Buffer {
  const encodedJSON = new TextEncoder().encode(JSON.stringify(document));
  const jsonLength = align4(encodedJSON.length);
  const hasBinary = binary !== undefined;
  const binaryLength = hasBinary ? align4(binary.byteLength) : 0;
  const totalLength = 12 + 8 + jsonLength + (hasBinary ? 8 + binaryLength : 0);
  const result = new Uint8Array(totalLength);
  const view = new DataView(result.buffer);

  view.setUint32(0, 0x46546c67, true);
  view.setUint32(4, 2, true);
  view.setUint32(8, totalLength, true);
  view.setUint32(12, jsonLength, true);
  view.setUint32(16, 0x4e4f534a, true);
  result.fill(0x20, 20, 20 + jsonLength);
  result.set(encodedJSON, 20);

  if (binary) {
    const header = 20 + jsonLength;
    view.setUint32(header, binaryLength, true);
    view.setUint32(header + 4, 0x004e4942, true);
    result.set(binary, header + 8);
  }
  return Buffer.from(result);
}

function align4(value: number): number {
  return (value + 3) & ~3;
}
