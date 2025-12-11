
class GuestFingerprint {
  // 1. 持久化存储配置
  static STORAGE_KEY = "guest_unique_id_v3"; // 版本号避免兼容问题
  static EXPIRE_DAYS = 365 * 2; // 持久化2年

  // 2. 生成/获取主UUID（核心标识，持久化）
  static getMainUUID() {
    let stored = localStorage.getItem(this.STORAGE_KEY);
    if (stored) {
      try {
        const { uuid, expire } = JSON.parse(stored);
        // 未过期则直接返回
        if (Date.now() < expire) return uuid;
      } catch (e) { /* 解析失败则重新生成 */ }
    }

    // 生成新UUID（浏览器原生API，无依赖）
    const uuid = crypto.randomUUID ? crypto.randomUUID() : this.fallbackUUID();
    // 设置过期时间
    const expire = Date.now() + this.EXPIRE_DAYS * 24 * 60 * 60 * 1000;
    // 持久化到localStorage（Cookie兜底，防止localStorage被禁）
    localStorage.setItem(this.STORAGE_KEY, JSON.stringify({ uuid, expire }));
    this.setCookie(this.STORAGE_KEY, JSON.stringify({ uuid, expire }), this.EXPIRE_DAYS);

    return uuid;
  }

  // 降级生成UUID（兼容旧浏览器）
  static fallbackUUID() {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
      const r = Math.random() * 16 | 0;
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16);
    });
  }

  // Cookie兜底存储（localStorage失效时）
  static setCookie(name, value, days) {
    const date = new Date();
    date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
    const expires = "; expires=" + date.toUTCString();
    document.cookie = name + "=" + (value || "")  + expires + "; path=/; SameSite=Lax";
  }

  // 3. 核心：获取平台/系统信息（替换废弃的 navigator.platform）
  static getPlatformInfo() {
    const info = {
      osName: "unknown",
      osVersion: "unknown",
      deviceType: "unknown"
    };

    // 方案1：现代浏览器优先用 userAgentData（结构化数据，无废弃风险）
    if (navigator.userAgentData && navigator.userAgentData.platform) {
      const { platform, osVersion } = navigator.userAgentData;
      info.osVersion = osVersion || "unknown";
      
      switch (platform.toLowerCase()) {
        case "windows":
          info.osName = "Windows";
          info.deviceType = "PC";
          break;
        case "macos":
          info.osName = "macOS";
          info.deviceType = "PC";
          break;
        case "android":
          info.osName = "Android";
          info.deviceType = "Mobile";
          break;
        case "ios":
          info.osName = "iOS";
          info.deviceType = "Mobile";
          break;
        case "linux":
          info.osName = "Linux";
          info.deviceType = "PC";
          break;
        default:
          info.osName = platform;
          info.deviceType = "PC";
      }
    } else {
      // 方案2：降级从 userAgent 提取（兼容所有浏览器）
      const ua = navigator.userAgent.toLowerCase();
      // 识别系统名称
      if (ua.includes("windows")) {
        info.osName = "Windows";
        // 提取Windows版本
        if (ua.includes("windows nt 10.0")) info.osVersion = "10";
        else if (ua.includes("windows nt 6.3")) info.osVersion = "8.1";
        else info.osVersion = "unknown";
        info.deviceType = "PC";
      } else if (ua.includes("mac os x")) {
        info.osName = "macOS";
        const macVer = /mac os x (\d+)/.exec(ua);
        info.osVersion = macVer ? macVer[1] : "unknown";
        info.deviceType = "PC";
      } else if (ua.includes("android")) {
        info.osName = "Android";
        const androidVer = /android (\d+)/.exec(ua);
        info.osVersion = androidVer ? androidVer[1] : "unknown";
        info.deviceType = "Mobile";
      } else if (ua.includes("iphone") || ua.includes("ipad")) {
        info.osName = "iOS";
        const iosVer = /os (\d+)_/.exec(ua);
        info.osVersion = iosVer ? iosVer[1] : "unknown";
        info.deviceType = ua.includes("ipad") ? "Tablet" : "Mobile";
      } else if (ua.includes("linux")) {
        info.osName = "Linux";
        info.deviceType = "PC";
      }
    }

    // 补充设备类型校验（通过触摸点数）
    if (navigator.maxTouchPoints > 0 && info.deviceType === "PC") {
      info.deviceType = "Tablet"; // 二合一设备（如Surface）
    }

    return info;
  }

  // 4. 收集稳定特征
  static getStableFeatures() {
    const platformInfo = this.getPlatformInfo();

    // 筛选绝对稳定的特征（无屏幕分辨率）
    return {
      timezone: new Date().getTimezoneOffset(), // 时区偏移（稳定）
      language: navigator.language || navigator.userLanguage || "unknown", // 浏览器语言（稳定）
      osName: platformInfo.osName, // 系统名称（替代原platform）
      osVersion: platformInfo.osVersion, // 系统版本（补充）
      deviceType: platformInfo.deviceType, // 设备类型（PC/Mobile/Tablet）
      hardwareConcurrency: navigator.hardwareConcurrency || 0, // CPU核心数（稳定）
      deviceMemory: navigator.deviceMemory || 0, // 设备内存（稳定）
      maxTouchPoints: navigator.maxTouchPoints || 0, // 触摸屏点数（稳定）
    };
  }

  // 5. 生成融合特征哈希（防篡改，服务端会重新计算校验）
  static async generateFusionHash() {
    const features = this.getStableFeatures();
    // 固定顺序拼接（避免前端篡改特征顺序）
    const keys = ["timezone", "language", "osName", "osVersion", "deviceType", "hardwareConcurrency", "deviceMemory", "maxTouchPoints"];
    const featureStr = keys.map(key => features[key]).join("|");
    // SHA256哈希（不可逆，防特征泄露）
    const encoder = new TextEncoder();
    const hashBuffer = await crypto.subtle.digest("SHA-256", encoder.encode(featureStr));
    return Array.from(new Uint8Array(hashBuffer)).map(b => b.toString(16).padStart(2, "0")).join("");
  }

  // 6. 组装请求参数（每次请求携带）
  static async getRequestParams() {
    const mainUUID = this.getMainUUID();
    const fusionHash = await this.generateFusionHash();
    const features = this.getStableFeatures();
    return {
      guest_uuid: mainUUID,       // 主标识
      fusion_hash: fusionHash,    // 融合特征哈希（用于校验）
      stable_features: features   // 原始特征（服务端重新计算哈希）
    };
  }
}

// 挂载到window，方便全局调用
window.GuestFingerprint = GuestFingerprint;

// 导出模块
export default GuestFingerprint;