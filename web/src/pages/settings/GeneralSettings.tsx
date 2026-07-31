import { useTranslation } from 'react-i18next'
export default function GeneralSettings() {
  const { t } = useTranslation()
  return <div><h2>{t('nav.sub.general')}</h2></div>
}
