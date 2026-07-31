import { useTranslation } from 'react-i18next'
export default function ScanConfig() {
  const { t } = useTranslation()
  return <div><h2>{t('nav.sub.scanConfig')}</h2></div>
}
