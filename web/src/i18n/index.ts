import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import zh from './zh.json'
import en from './en.json'

export const LANGUAGE_KEY = 'sentinel.lang'

// 语言解析顺序(落实 Task 11 brief Interfaces,修 Task 15 e2e 暴露的缺陷):
//   localStorage(sentinel.lang) → 后端 config.language(fetchSettings 注入,见 store)→ 回退 zh
// 故 detection.order 只认 localStorage,不认 navigator。
// 原因:navigator 在 en-US 浏览器(含 Playwright headless chromium)会把首屏劫持成英文,
// 违反 spec「全站中文不变」与后端 config「空 language = 回退 zh」约定。英文仅由用户主动
// 切换(localStorage)或后端显式配置 language:en 触发。见 store fetchSettings 的后端层注入。
i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: { zh: { translation: zh }, en: { translation: en } },
    // lng 兜底:localStorage 无值时(首访 / 后端未注入前)默认中文,落实「空 = 回退 zh」。
    lng: 'zh',
    fallbackLng: 'zh',
    detection: {
      order: ['localStorage'],
      lookupLocalStorage: LANGUAGE_KEY,
      caches: ['localStorage'],
    },
    interpolation: { escapeValue: false },
  })

export default i18n
