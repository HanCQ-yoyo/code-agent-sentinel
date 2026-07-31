import { useTranslation } from 'react-i18next'
export default function ScanRules() {
  const { t } = useTranslation()
  return <div><h2>{t('nav.sub.scanRules')}</h2></div>
}
