import { useEffect, useRef, useState } from 'react'
import { useNotifications } from '../notifications/context'
import type { ChannelPreferences } from '../notifications/types'
import { NotificationFeedback } from '../components/notifications/NotificationFeedback'

export default function NotificationSettingsPage() {
  const { service, go } = useNotifications()
  const [channels, setChannels] = useState<ChannelPreferences | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<'saved' | 'error' | null>(null)
  const [attempt, setAttempt] = useState(0)
  const saveRequest = useRef<AbortController | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    service.getPreferences(controller.signal).then(value => {
      if (controller.signal.aborted) return
      setChannels(value)
      setStatus('ready')
    }).catch(() => {
      if (!controller.signal.aborted) setStatus('error')
    })
    return () => controller.abort()
  }, [service, attempt])

  useEffect(() => () => saveRequest.current?.abort(), [])

  async function save() {
    if (!channels || saving) return
    const controller = new AbortController()
    saveRequest.current?.abort()
    saveRequest.current = controller
    setSaving(true)
    setMessage(null)
    try {
      const saved = await service.savePreferences({
        email_enabled: channels.email_enabled, in_app_enabled: channels.in_app_enabled,
      }, controller.signal)
      if (controller.signal.aborted) return
      setChannels(saved)
      setMessage('saved')
    } catch {
      if (!controller.signal.aborted) setMessage('error')
    } finally {
      if (!controller.signal.aborted) setSaving(false)
    }
  }

  return (
    <section className="notification-page" aria-labelledby="notification-settings-title">
      <button
        className="notification-back"
        type="button"
        onClick={() => go('/app/notifications')}
      >← Back to notifications</button>
      <div className="notification-page-heading">
        <div>
          <h1 id="notification-settings-title">Notification settings</h1>
          <p>Choose where you receive your account updates.</p>
        </div>
      </div>
      {status !== 'ready' || !channels ? (
        <NotificationFeedback state={status === 'ready' ? 'error' : status}
          onRetry={() => {
            setStatus('loading')
            setAttempt(previous => previous + 1)
          }} />
      ) : <form
        className="notification-settings-card"
        onSubmit={event => { event.preventDefault(); void save() }}
      >
        <h2>Delivery channels</h2>
        <label className="notification-channel">
          <span><strong>Email notifications</strong>
            <span>Receive account updates at your verified email address.</span>
          </span>
          <input
            type="checkbox"
            checked={channels.email_enabled}
            disabled={saving}
            onChange={event => {
              setChannels({ ...channels, email_enabled: event.target.checked })
              setMessage(null)
            }}
          />
        </label>
        <label className="notification-channel">
          <span><strong>In-app notifications</strong>
            <span>Show ordinary account updates in your GovAI inbox.</span>
          </span>
          <input
            type="checkbox"
            checked={channels.in_app_enabled}
            disabled={saving}
            onChange={event => {
              setChannels({ ...channels, in_app_enabled: event.target.checked })
              setMessage(null)
            }}
          />
        </label>
        <p className="notification-policy">
          Security alerts about sign-ins and password changes always appear in-app,
          even when ordinary in-app updates are off. Your email choice still applies.
        </p>
        {message === 'saved' && <p role="status">Notification settings saved.</p>}
        {message === 'error' && <p role="alert">
          We couldn’t save your settings. Please try again.
        </p>}
        <button className="notification-primary" type="submit" disabled={saving}>
          {saving ? 'Saving…' : 'Save changes'}
        </button>
      </form>}
    </section>
  )
}
