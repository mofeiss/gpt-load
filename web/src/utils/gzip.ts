import { inflate } from "pako";

/**
 * 尝试解压 gzip 压缩的字符串
 * @param str - 可能被 gzip 压缩的字符串
 * @returns 解压后的字符串，如果解压失败则返回原始字符串
 */
export function tryGzipDecode(str: string): string {
  if (!str || str.trim() === "") {
    return str;
  }

  // 1. 首先检查是否看起来像 JSON，如果是则直接返回（未压缩）
  const trimmed = str.trim();
  if (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  ) {
    return str;
  }

  // 2. 尝试检测是否为 base64 编码的 gzip 数据
  try {
    // 检查是否为有效的 base64 字符串
    if (!/^[A-Za-z0-9+/]*={0,2}$/.test(str)) {
      return str;
    }

    // 尝试 base64 解码
    const binaryString = atob(str);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }

    // 检查 gzip 头部标识 (1f 8b)
    if (bytes.length < 2 || bytes[0] !== 0x1f || bytes[1] !== 0x8b) {
      return str;
    }

    // 尝试 gzip 解压
    const decompressed = inflate(bytes, { to: "string" });
    return decompressed;
  } catch (_error) {
    // 3. 如果 base64 + gzip 方式失败，尝试直接解压（二进制数据）
    try {
      const bytes = new Uint8Array(str.length);
      for (let i = 0; i < str.length; i++) {
        bytes[i] = str.charCodeAt(i);
      }

      // 检查 gzip 头部标识
      if (bytes.length >= 2 && bytes[0] === 0x1f && bytes[1] === 0x8b) {
        const decompressed = inflate(bytes, { to: "string" });
        return decompressed;
      }
    } catch (_innerError) {
      // 忽略内部错误，返回原始字符串
    }

    // 如果所有解压尝试都失败，返回原始字符串
    return str;
  }
}

/**
 * 检测字符串是否可能是 gzip 压缩的数据
 * @param str - 待检测的字符串
 * @returns 是否可能是 gzip 数据
 */
export function isLikelyGzipData(str: string): boolean {
  if (!str || str.length < 10) {
    return false;
  }

  // 检查是否是明显的 JSON
  const trimmed = str.trim();
  if (
    (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
    (trimmed.startsWith("[") && trimmed.endsWith("]"))
  ) {
    return false;
  }

  // 检查是否包含大量非打印字符或 base64 模式
  const nonPrintableChars = str.match(/[^\x20-\x7E]/g);
  const isBase64Like = /^[A-Za-z0-9+/]+=*$/.test(str.replace(/\s/g, ""));

  return (nonPrintableChars && nonPrintableChars.length > str.length * 0.3) || isBase64Like;
}
