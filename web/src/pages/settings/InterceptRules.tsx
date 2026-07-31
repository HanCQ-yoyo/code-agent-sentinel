import { useTranslation } from 'react-i18next'
export default function InterceptRules() {
  const { t } = useTranslation()
  return <div><h2>{t('nav.sub.interceptRules')}</h2></div>
}
