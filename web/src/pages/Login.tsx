import {
  Activity,
  ArrowLeft,
  Boxes,
  Eye,
  EyeOff,
  Languages,
  Lock,
  LockKeyhole,
  Server as ServerIcon,
  ShieldCheck,
  User,
  Zap,
} from 'lucide-react';
import { useState, type FormEvent } from 'react';
import { api, setSession as setApiSession } from '../api';
import { ActionButton } from '../components/ui/ActionButton';
import { useI18n } from '../i18n';
import type { Session } from '../types';
import portalNight from '../assets/minecraft/portal-night.jpg';
import './Login.css';

export function Login({ onLogin }: { onLogin: (session: Session) => void }) {
  const { locale, setLocale, tr } = useI18n();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setBusy(true);
    setError('');
    const data = new FormData(event.currentTarget);
    try {
      const session = await api<Session>(registering ? '/api/v1/auth/register' : '/api/v1/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          username: data.get('username'),
          password: data.get('password'),
        }),
      });
      setApiSession(session);
      onLogin(session);
    } catch (value) {
      setError(value instanceof Error ? value.message : tr('Login gagal.', 'Login failed.'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="login-wrapper mc-theme">
      {/* Background Image with Cinematic Overlay */}
      <div className="login-bg-media">
        <img src={portalNight} alt="Minecraft Nether Portal Night" fetchPriority="high" />
        <div className="login-bg-overlay" />
      </div>

      <main className="login-screen">
        {/* Top bar controls */}
        <div className="login-top-bar">
          <a href="/" className="login-back-btn">
            <ArrowLeft />
            <span>{tr('Kembali ke Beranda', 'Back to Home')}</span>
          </a>
          <button
            className="login-language language-toggle"
            type="button"
            onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}
            aria-label={tr('Ganti bahasa', 'Switch language')}
          >
            <Languages />
            <span>{locale.toUpperCase()}</span>
          </button>
        </div>

        {/* Left Side: Minecraft Showcase & Branding */}
        <section className="login-intro">
          <div className="login-brand">
            <span className="brand-mark mc-brand-cube">
              <span className="mc-grass-top" />
              <ServerIcon />
            </span>
            <div className="login-brand-copy">
              <b>MyPanel</b>
              <span className="login-version-pill">v2.0</span>
            </div>
          </div>

          <div className="login-eyebrow-pill">
            <span className="live-dot" />
            <span>MINECRAFT HOSTING PANEL</span>
          </div>

          <h1>{tr('Server Anda. Infrastruktur Kami.', 'Your server. Our infrastructure.')}</h1>

          <p className="login-intro-lead">
            {tr(
              'Kelola server Minecraft, pantau performa real-time 20.0 TPS, dan instal plugin dengan satu klik.',
              'Manage Minecraft servers, monitor 20.0 TPS telemetry in real-time, and install plugins with one click.'
            )}
          </p>

          <div className="login-feature-list">
            <div className="login-feature-item">
              <div className="feature-item-icon">
                <Zap />
              </div>
              <div>
                <strong>{tr('Deploy Instan < 3 Detik', 'Instant < 3s Deployment')}</strong>
                <p>{tr('Paper, Purpur, Fabric siap dimainkan seketika.', 'Paper, Purpur, Fabric ready to play instantly.')}</p>
              </div>
            </div>

            <div className="login-feature-item">
              <div className="feature-item-icon">
                <ShieldCheck />
              </div>
              <div>
                <strong>{tr('Isolasi Container Aman', 'Isolated Container Sandbox')}</strong>
                <p>{tr('Batas cgroup mandiri tanpa pengaruh server lain.', 'Dedicated cgroup limits with zero interference.')}</p>
              </div>
            </div>

            <div className="login-feature-item">
              <div className="feature-item-icon">
                <Activity />
              </div>
              <div>
                <strong>{tr('Performa 20.0 TPS & NVMe', 'Stable 20.0 TPS & NVMe Storage')}</strong>
                <p>{tr('Jaminan uptime dan kapasitas terukur.', 'Guaranteed uptime and verified node capacity.')}</p>
              </div>
            </div>
          </div>

          <div className="login-cluster-status">
            <span className="live-dot" />
            <span>
              {tr(
                'Cluster Node Status: Online (100% Siap)',
                'Cluster Node Status: Online (100% Operational)'
              )}
            </span>
          </div>
        </section>

        {/* Right Side: Obsidian Glassmorphic Login Form */}
        <div className="login-card-container">
          <div className="login-card-ambient" />
          <form className="surface login-card" onSubmit={submit}>
            <div className="login-card-header">
              <div className="login-header-icon">
                <LockKeyhole />
              </div>
              <div className="login-header-text">
                <h2>{registering ? tr('Buat akun', 'Create account') : tr('Masuk ke MyPanel', 'Sign in to MyPanel')}</h2>
                <p>
                  {registering
                    ? tr('Daftar menggunakan username dan password.', 'Register with a username and password.')
                    : tr('Admin dan user masuk melalui halaman yang sama.', 'Admins and users sign in on the same page.')}
                </p>
              </div>
            </div>

            {error && (
              <div className="inline-error mc-inline-error" role="alert">
                <span className="error-indicator">!</span>
                <span>{error}</span>
              </div>
            )}

            <div className="login-fields">
              <label>
                Username
                <div className="input-with-icon">
                  <User className="field-icon" />
                  <input
                    name="username"
                    autoComplete="username"
                    minLength={3}
                    maxLength={32}
                    pattern="[A-Za-z0-9_.-]+"
                    placeholder={tr('contoh: admin atau player1', 'e.g. admin or player1')}
                    required
                    autoFocus
                  />
                </div>
              </label>

              <label>
                Password
                <div className="input-with-icon">
                  <Lock className="field-icon" />
                  <input
                    name="password"
                    type={showPassword ? 'text' : 'password'}
                    minLength={12}
                    autoComplete={registering ? 'new-password' : 'current-password'}
                    placeholder="••••••••••••"
                    required
                  />
                  <button
                    type="button"
                    className="password-toggle-btn"
                    onClick={() => setShowPassword(!showPassword)}
                    tabIndex={-1}
                    aria-label={showPassword ? tr('Sembunyikan password', 'Hide password') : tr('Tampilkan password', 'Show password')}
                  >
                    {showPassword ? <EyeOff /> : <Eye />}
                  </button>
                </div>
              </label>
            </div>

            <ActionButton type="submit" size="lg" loading={busy} className="mc-btn-emerald login-submit-btn">
              {registering ? tr('Daftar', 'Register') : tr('Masuk', 'Sign in')}
            </ActionButton>

            <button
              type="button"
              className="text-button login-switch"
              onClick={() => {
                setRegistering(!registering);
                setError('');
              }}
            >
              {registering
                ? tr('Sudah punya akun? Masuk', 'Already registered? Sign in')
                : tr('Belum punya akun? Daftar', 'No account yet? Register')}
            </button>

            <div className="login-quick-hint">
              <span>
                💡 {tr('Masuk dengan akun Anda atau klik Daftar untuk membuat akun baru.', 'Sign in with your account or click Register to create a new one.')}
              </span>
            </div>
          </form>
        </div>
      </main>
    </div>
  );
}
