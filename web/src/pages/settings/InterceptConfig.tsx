import { useTranslation } from 'react-i18next'
export default function InterceptConfig() {
  const { t } = useTranslation()
  return <div><h2>{t('nav.sub.interceptConfig')}</h2></div>
}
