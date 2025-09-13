import { copy } from "./clipboard";

// 日志队列
const logQueue: string[] = [];
let isExporterSetup = false;

/**
 * 记录日志并推入队列
 * @param message 日志消息
 * @param data 相关数据
 */
export function log(message: string, data?: any) {
  const timestamp = new Date().toISOString();
  const logMessage = `[${timestamp}] ${message}`;

  // 打印到控制台以便实时查看
  if (data !== undefined) {
    console.log(logMessage, data);
  } else {
    console.log(logMessage);
  }

  // 准备推入队列的完整日志字符串
  let fullLogEntry = logMessage;
  if (data !== undefined) {
    try {
      // 美化JSON输出
      fullLogEntry += `\n${JSON.stringify(data, null, 2)}`;
    } catch (_e) {
      fullLogEntry += `\n[Unserializable data]`;
    }
  }
  logQueue.push(fullLogEntry);
}

/**
 * 设置全局的日志导出函数 window.copylog
 */
export function setupGlobalLogExporter() {
  if (isExporterSetup) {
    return;
  }

  (window as any).copylog = () => {
    if (logQueue.length === 0) {
      console.log("%c[Logger] Log queue is empty.", "color: orange;");
      return;
    }

    const logContent = logQueue.join("\n\n==============================\n\n");

    console.log(
      "%c[Logger] Copying all logs to clipboard and printing below:",
      "color: lightblue;"
    );
    console.log(logContent);

    copy(logContent).then(success => {
      if (success) {
        console.log("%c[Logger] Logs copied to clipboard successfully!", "color: lightgreen;");
      } else {
        console.error("%c[Logger] Failed to copy logs to clipboard.", "color: red;");
      }
    });

    // 清空队列
    logQueue.length = 0;
    console.log("%c[Logger] Log queue has been cleared.", "color: lightblue;");
  };

  isExporterSetup = true;
  console.log(
    "%c[Logger] `window.copylog()` is now available. Call it to copy and clear the log queue.",
    "color: lightgreen;"
  );
}
