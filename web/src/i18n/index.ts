import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import zh from './zh.json'
import en from './en.json'

export const LANGUAGE_KEY = 'sentinel.lang'

// 语言解析顺序(三类来源,优先级从高到低):
//   1. localStorage(sentinel.lang):用户主动切换写入,最高优先,跨刷新保留。
//   2. 后端 config.language:fetchSettings 注入(见 store),跨重启/跨端口持久化。
//   3. 默认 en(英文):无任何来源时兜底。
//
// 关键:这里**不**显式传 `lng`,让 i18next 的 detection 机制生效(order: ['localStorage'])。
// 上一版传了 `lng: 'zh'`,i18next 的 changeLanguage 在 init 时检测到 options.lng 已被显式
// 设值后会跳过 detector(见 i18next changeLanguage 源码:`!lng && languageDetector` 分支),
// 导致 localStorage 里的用户偏好在刷新后被忽略、每次都回落到 'zh'——即「切换语种刷新后
// 恢复默认」bug。移除显式 lng 后:detector 先查 localStorage,有值则用之(持久化生效),
// 无值则 changeLanguage(undefined) → getBestMatchFromCodes 落 fallbackLng=en。
//
// caches:['localStorage']:用户主动 changeLanguage 时回写 localStorage,确保切换持久。
// 后端 config.language 层不写 localStorage(见 store fetchSettings:仅 localStorage 缺失时
// 才 changeLanguage 到后端值,不缓存),避免后端配置覆盖用户的本地偏好。
i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { zh: { translation: zh }, en: { translation: en } },
    // 兜底语言:无任何来源(首次访问 + 后端未配置)时显示英文(默认英文需求)。
    fallbackLng: 'en',
    supportedLngs: ['zh', 'en'],
    detection: {
      order: ['localStorage'],
      lookupLocalStorage: LANGUAGE_KEY,
      caches: ['localStorage'],
    },
    interpolation: { escapeValue: false },
  })

export default i18n
