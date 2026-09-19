import {
  Activity,
  ArrowUpRight,
  Boxes,
  Check,
  CheckCircle2,
  ChevronRight,
  CloudCog,
  Copy,
  Cpu,
  FileCode,
  FolderArchive,
  HardDrive,
  Languages,
  Menu,
  Moon,
  Server,
  ShieldCheck,
  Sparkles,
  SquareTerminal,
  Sun,
  Users,
  X,
} from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useI18n } from '../i18n';
import heroLandscape from '../assets/minecraft/hero-landscape.jpg';
import redstoneDatacenter from '../assets/minecraft/redstone-datacenter.jpg';
import './Landing.css';

export function Landing() {
  const { locale, setLocale, tr } = useI18n();
  const [menuOpen, setMenuOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [selectedRuntime, setSelectedRuntime] = useState<'paper' | 'purpur' | 'fabric' | 'vanilla'>('paper');
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'light' ? 'light' : 'dark');

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('mypanel.theme', next);
    setTheme(next);
  };

  const closeMenu = () => setMenuOpen(false);

  const handleCopy = () => {
    navigator.clipboard?.writeText('smp.mypanel.net:25565');
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="landing-page mc-theme">
      <a className="landing-skip" href="#main">
        {tr('Lewati ke konten', 'Skip to content')}
      </a>

      {/* Navigation */}
      <header className="landing-nav">
        <Link className="landing-wordmark" to="/" aria-label="MyPanel home">
          <span className="brand-mark mc-brand-cube">
            <span className="mc-grass-top" />
            <Server />
          </span>
          <div className="landing-brand-text">
            <strong>MyPanel</strong>
            <span className="landing-badge">V2</span>
          </div>
        </Link>

        <nav
          className={menuOpen ? 'landing-links open' : 'landing-links'}
          aria-label={tr('Navigasi utama', 'Main navigation')}
        >
          <a href="#control" onClick={closeMenu}>
            {tr('Kontrol', 'Control')}
          </a>
          <a href="#infrastructure" onClick={closeMenu}>
            {tr('Infrastruktur', 'Infrastructure')}
          </a>
          <a href="#workflow" onClick={closeMenu}>
            {tr('Cara kerja', 'How it works')}
          </a>
          <Link className="landing-mobile-login" to="/login" onClick={closeMenu}>
            {tr('Masuk ke panel', 'Open panel')}
            <ChevronRight />
          </Link>
        </nav>

        <div className="landing-nav-actions">
          <button
            className="landing-icon-button"
            type="button"
            onClick={() => setLocale(locale === 'id' ? 'en' : 'id')}
            aria-label={tr('Gunakan bahasa Inggris', 'Use Indonesian')}
          >
            <Languages />
            <span>{locale.toUpperCase()}</span>
          </button>
          <button
            className="landing-icon-button theme"
            type="button"
            onClick={toggleTheme}
            aria-label={
              theme === 'dark'
                ? tr('Gunakan mode terang', 'Use light mode')
                : tr('Gunakan mode gelap', 'Use dark mode')
            }
          >
            {theme === 'dark' ? <Sun /> : <Moon />}
          </button>
          <Link className="landing-login mc-btn-glow" to="/login">
            {tr('Masuk', 'Sign in')}
            <ArrowUpRight />
          </Link>
          <button
            className="landing-menu-button"
            type="button"
            onClick={() => setMenuOpen((value) => !value)}
            aria-expanded={menuOpen}
            aria-label={menuOpen ? tr('Tutup menu', 'Close menu') : tr('Buka menu', 'Open menu')}
          >
            {menuOpen ? <X /> : <Menu />}
          </button>
        </div>
      </header>

      <main id="main">
        {/* Hero Section */}
        <section className="landing-hero">
          <div className="landing-hero-copy">
            <div className="landing-eyebrow-pill">
              <span className="live-dot" />
              <span>MINECRAFT SERVER CONTROL PLANE</span>
            </div>

            <h1>{tr('Host Minecraft. Tanpa ribet.', 'Host Minecraft. Skip busywork.')}</h1>

            <p className="landing-hero-desc">
              {tr(
                'Beli kapasitas, jalankan server Paper, Purpur, atau Fabric dalam hitungan detik, dan pantau performa 20.0 TPS langsung dari browser.',
                'Buy capacity, launch a Paper, Purpur, or Fabric server in seconds, and track real-time 20.0 TPS performance directly from your browser.'
              )}
            </p>

            <div className="landing-hero-actions">
              <Link className="landing-primary mc-btn-emerald" to="/login">
                {tr('Mulai sekarang', 'Get started')}
                <ChevronRight />
              </Link>
              <a className="landing-secondary" href="#workflow">
                {tr('Lihat cara kerja', 'See how it works')}
              </a>
            </div>

            <div className="landing-hero-highlights">
              <div className="landing-highlight-item">
                <ShieldCheck />
                <span>{tr('Anti-DDoS & Sandbox Container', 'Anti-DDoS & Container Sandbox')}</span>
              </div>
              <div className="landing-highlight-item">
                <Sparkles />
                <span>{tr('Modrinth & CurseForge 1-Click', '1-Click Modrinth & Plugins')}</span>
              </div>
              <div className="landing-highlight-item">
                <Activity />
                <span>{tr('20.0 TPS Stabil & Jaminan NVMe', 'Stable 20.0 TPS & NVMe Storage')}</span>
              </div>
            </div>
          </div>

          {/* Interactive Minecraft Server Card Mockup */}
          <div className="landing-hero-showcase">
            <div className="showcase-card-ambient" />
            <div className="showcase-card">
              {/* Card Banner with Generated Artwork */}
              <div className="showcase-banner">
                <img
                  src={heroLandscape}
                  alt={tr('Pemandangan kastil Minecraft di pulau melayang', 'Cinematic Minecraft floating island castle')}
                  fetchPriority="high"
                />
                <div className="showcase-banner-overlay" />
                <div className="showcase-banner-content">
                  <div className="showcase-status-badge">
                    <span className="live-dot" />
                    <span>ONLINE</span>
                  </div>
                  <span className="showcase-version-badge">Paper 1.21.1</span>
                </div>
              </div>

              {/* Card Header */}
              <div className="showcase-body">
                <div className="showcase-server-title">
                  <div className="showcase-server-info">
                    <h3>Craftopia Survival SMP</h3>
                    <p className="showcase-motd">
                      §a§lValhalla SMP §7| §eNo Lag §7• §bCustom Enchants §7• §a1.21.1
                    </p>
                  </div>
                  <button
                    className={`showcase-ip-badge ${copied ? 'copied' : ''}`}
                    onClick={handleCopy}
                    type="button"
                    title={tr('Klik untuk salin alamat IP', 'Click to copy server IP')}
                  >
                    <code>smp.mypanel.net:25565</code>
                    {copied ? <Check /> : <Copy />}
                  </button>
                </div>

                {/* Telemetry Metrics Bar */}
                <div className="showcase-metrics-grid">
                  <div className="showcase-metric">
                    <div className="showcase-metric-head">
                      <span className="metric-label">
                        <Activity /> TPS
                      </span>
                      <strong className="metric-val text-emerald">20.0</strong>
                    </div>
                    <div className="showcase-progress">
                      <div className="showcase-progress-bar bg-emerald" style={{ width: '100%' }} />
                    </div>
                  </div>

                  <div className="showcase-metric">
                    <div className="showcase-metric-head">
                      <span className="metric-label">
                        <Users /> PLAYERS
                      </span>
                      <strong className="metric-val">34 / 60</strong>
                    </div>
                    <div className="showcase-progress">
                      <div className="showcase-progress-bar bg-sky" style={{ width: '56%' }} />
                    </div>
                  </div>

                  <div className="showcase-metric">
                    <div className="showcase-metric-head">
                      <span className="metric-label">
                        <Cpu /> MEMORY
                      </span>
                      <strong className="metric-val">3.8 / 8.0 GB</strong>
                    </div>
                    <div className="showcase-progress">
                      <div className="showcase-progress-bar bg-amber" style={{ width: '47%' }} />
                    </div>
                  </div>
                </div>

                {/* Live Terminal Stream Snippet */}
                <div className="showcase-terminal">
                  <div className="terminal-header">
                    <span className="terminal-dots">
                      <span />
                      <span />
                      <span />
                    </span>
                    <span className="terminal-title">live console stream</span>
                    <span className="terminal-chip">ansi 256</span>
                  </div>
                  <div className="terminal-lines">
                    <p className="terminal-line">
                      <span className="t-time">[14:20:00]</span> <span className="t-info">[INFO]</span>{' '}
                      <span className="t-msg">Preparing spawn area: 100% (world, nether, the_end)</span>
                    </p>
                    <p className="terminal-line">
                      <span className="t-time">[14:20:01]</span> <span className="t-info">[INFO]</span>{' '}
                      <span className="t-msg">
                        Running on <span className="t-highlight">Paper 1.21.1-R0.1</span> (Java 21 OpenJDK)
                      </span>
                    </p>
                    <p className="terminal-line">
                      <span className="t-time">[14:20:04]</span> <span className="t-green">[JOIN]</span>{' '}
                      <span className="t-player">Alex</span> connected from 192.168.1.102
                    </p>
                    <p className="terminal-line">
                      <span className="t-time">[14:20:06]</span> <span className="t-cyan">[SPARK]</span>{' '}
                      <span className="t-msg">TPS: 20.00 | MSPT: 11.2ms | GC pause: 0.0ms</span>
                    </p>
                  </div>
                </div>

                {/* Simulated Quick Action Bar */}
                <div className="showcase-controls">
                  <div className="showcase-action-tabs">
                    <button
                      type="button"
                      className={selectedRuntime === 'paper' ? 'active' : ''}
                      onClick={() => setSelectedRuntime('paper')}
                    >
                      Paper
                    </button>
                    <button
                      type="button"
                      className={selectedRuntime === 'purpur' ? 'active' : ''}
                      onClick={() => setSelectedRuntime('purpur')}
                    >
                      Purpur
                    </button>
                    <button
                      type="button"
                      className={selectedRuntime === 'fabric' ? 'active' : ''}
                      onClick={() => setSelectedRuntime('fabric')}
                    >
                      Fabric
                    </button>
                    <button
                      type="button"
                      className={selectedRuntime === 'vanilla' ? 'active' : ''}
                      onClick={() => setSelectedRuntime('vanilla')}
                    >
                      Vanilla
                    </button>
                  </div>
                  <span className="showcase-pill-ready">
                    <CheckCircle2 /> {tr('Siap dalam 2.8 detik', 'Ready in 2.8s')}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Runtime Support Strip */}
        <section className="landing-runtime-strip" aria-label={tr('Runtime yang didukung', 'Supported runtimes')}>
          <span>{tr('Didukung sepenuhnya untuk', 'Fully supported for')}</span>
          <div className="runtime-badge">
            <span className="runtime-indicator paper" />
            <strong>PaperMC</strong>
          </div>
          <div className="runtime-badge">
            <span className="runtime-indicator purpur" />
            <strong>Purpur</strong>
          </div>
          <div className="runtime-badge">
            <span className="runtime-indicator fabric" />
            <strong>Fabric</strong>
          </div>
          <div className="runtime-badge">
            <span className="runtime-indicator vanilla" />
            <strong>Vanilla</strong>
          </div>
        </section>

        {/* Control Section / Bento Grid */}
        <section className="landing-control" id="control">
          <header>
            <div className="section-tag">
              <Sparkles />
              <span>FITUR UTAMA</span>
            </div>
            <h2>{tr('Yang penting terlihat. Yang rumit tetap di belakang.', 'See what matters. Keep complexity behind the scenes.')}</h2>
            <p>
              {tr(
                'Console, file, backup, jadwal, dan metrik memakai alur yang sama di setiap server Minecraft Anda.',
                'Console, files, backups, schedules, and metrics follow the same flow on every server.'
              )}
            </p>
          </header>

          <div className="landing-feature-grid">
            <article className="landing-feature feature-console">
              <div className="feature-icon-wrapper">
                <SquareTerminal />
              </div>
              <div className="feature-content">
                <h3>{tr('Console langsung & Realtime', 'Live terminal & streaming')}</h3>
                <p>
                  {tr(
                    'Log berwarna ANSI dan eksekusi command instan melalui pipe tanpa menunggu buffering batch.',
                    'Colored ANSI logs and instant command execution over direct pipes with zero buffering delay.'
                  )}
                </p>
                <div className="mini-terminal-preview">
                  <code>&gt; /op Steve</code>
                  <span className="t-green">Made Steve a server operator</span>
                </div>
              </div>
            </article>

            <article className="landing-feature feature-files">
              <div className="feature-icon-wrapper">
                <FileCode />
              </div>
              <div className="feature-content">
                <h3>{tr('File tanpa pindah alat', 'Files without switching tools')}</h3>
                <p>
                  {tr(
                    'Edit server.properties, kelola folder world, dan unggah plugin .jar langsung di browser dengan editor modern.',
                    'Edit server.properties, manage world folders, and upload .jar plugins right from your browser.'
                  )}
                </p>
                <div className="file-chips">
                  <span>server.properties</span>
                  <span>spigot.yml</span>
                  <span>plugins/</span>
                </div>
              </div>
            </article>

            <article className="landing-feature feature-metrics">
              <div className="feature-icon-wrapper">
                <Activity />
              </div>
              <div className="feature-content">
                <h3>{tr('Metrik & TPS akurat', 'Accurate telemetry & TPS')}</h3>
                <p>
                  {tr(
                    'Pantau CPU core, pemakaian heap RAM, dan I/O disk real-time sesuai batas cgroup container aktual.',
                    'Monitor CPU cores, heap RAM allocation, and disk I/O in real time strictly tied to container cgroups.'
                  )}
                </p>
                <div className="metric-badge-row">
                  <span className="m-pill">TPS: 20.0</span>
                  <span className="m-pill">RAM: 98% Eff</span>
                  <span className="m-pill">NVMe I/O</span>
                </div>
              </div>
            </article>

            <article className="landing-feature feature-visual">
              <img
                src={redstoneDatacenter}
                width="1400"
                height="787"
                loading="lazy"
                alt={tr('Diorama datacenter server Minecraft berteknologi tinggi dengan obsidian dan redstone', 'High-tech Minecraft server rack datacenter built with obsidian and redstone')}
              />
              <div className="feature-visual-overlay">
                <span className="visual-badge">
                  <CloudCog /> {tr('Arsitektur Container Terisolasi', 'Isolated Container Sandbox')}
                </span>
              </div>
            </article>
          </div>
        </section>

        {/* Infrastructure Section */}
        <section className="landing-infrastructure" id="infrastructure">
          <figure className="infra-figure">
            <img
              src={redstoneDatacenter}
              width="1400"
              height="787"
              loading="lazy"
              alt={tr('Infrastruktur server datacenter Minecraft', 'Minecraft server datacenter infrastructure')}
            />
            <div className="infra-glow" />
          </figure>

          <div className="infra-copy">
            <div className="section-tag">
              <ShieldCheck />
              <span>INFRASTRUKTUR HANDAL</span>
            </div>
            <h2>{tr('Kapasitas dijaga sebelum server dibeli.', 'Capacity is checked before a server is purchased.')}</h2>
            <p>
              {tr(
                'MyPanel memeriksa alokasi CPU, RAM, dan disk sebelum pembuatan server agar paket baru tidak melebihi batas node atau menurunkan TPS server lain.',
                'MyPanel validates CPU, RAM, and disk allocations before server launch so new instances never exceed node capacity or compromise TPS.'
              )}
            </p>

            <div className="landing-infra-points">
              <div className="infra-point-card">
                <ShieldCheck />
                <div>
                  <strong>{tr('Akses admin & user terpisah', 'Role-based access control')}</strong>
                  <p>{tr('Admin mengelola host dan paket, user menikmati kontrol penuh atas servernya.', 'Admins manage hosts and pricing; users get dedicated sovereignty over their servers.')}</p>
                </div>
              </div>

              <div className="infra-point-card">
                <Boxes />
                <div>
                  <strong>{tr('Server berjalan dalam container', 'Docker container isolation')}</strong>
                  <p>{tr('Setiap instance terisolasi dengan cgroup limits sendiri tanpa risiko crash silang.', 'Each instance is fully sandboxed with independent cgroups and zero cross-crash risk.')}</p>
                </div>
              </div>

              <div className="infra-point-card">
                <FolderArchive />
                <div>
                  <strong>{tr('Backup terjadwal & snapshot instan', 'Automated scheduled backups')}</strong>
                  <p>{tr('Amankan world, database, dan plugin Anda ke archive terkompresi dengan 1 klik.', 'Protect your worlds, configs, and plugins into compressed archives with 1-click restore.')}</p>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Workflow Section */}
        <section className="landing-workflow" id="workflow">
          <div className="landing-workflow-copy">
            <div className="section-tag">
              <Sparkles />
              <span>LANGKAH MUDAH</span>
            </div>
            <h2>{tr('Dari paket ke dunia baru dalam satu alur.', 'From a package to a new world in one flow.')}</h2>
            <p>
              {tr(
                'Tidak perlu berpindah dashboard atau menyalin konfigurasi manual. Semua siap dimainkan.',
                'No dashboard hopping or manual configuration copying required. Jump right into your world.'
              )}
            </p>
          </div>

          <ol className="workflow-steps">
            <li className="workflow-step">
              <div className="step-number">01</div>
              <div className="step-card">
                <div className="step-icon"><HardDrive /></div>
                <h3>{tr('Pilih kapasitas', 'Choose capacity')}</h3>
                <p>{tr('Tentukan alokasi RAM, CPU core, dan disk NVMe sesuai kebutuhan pemain Anda.', 'Pick RAM, CPU cores, and NVMe disk storage tailored to your community size.')}</p>
              </div>
            </li>

            <li className="workflow-step">
              <div className="step-number">02</div>
              <div className="step-card">
                <div className="step-icon"><Cpu /></div>
                <h3>{tr('Tentukan runtime', 'Select a runtime')}</h3>
                <p>{tr('Pilih Paper, Purpur, Fabric, atau Vanilla beserta versi Minecraft yang diinginkan.', 'Choose Paper, Purpur, Fabric, or Vanilla along with your desired Minecraft version.')}</p>
              </div>
            </li>

            <li className="workflow-step">
              <div className="step-number">03</div>
              <div className="step-card">
                <div className="step-icon"><Server /></div>
                <h3>{tr('Kelola dari panel', 'Manage from the panel')}</h3>
                <p>{tr('Java 21/17 disesuaikan otomatis, port dialokasikan, dan console siap dimainkan.', 'Java runtime auto-configured, ports provisioned, and live console ready to play.')}</p>
              </div>
            </li>
          </ol>
        </section>

        {/* Call to action */}
        <section className="landing-cta mc-cta-box">
          <div className="cta-glow-effect" />
          <div className="cta-content">
            <span className="brand-mark mc-brand-cube">
              <span className="mc-grass-top" />
              <Server />
            </span>
            <h2>{tr('Server Anda berikutnya dimulai di sini.', 'Your next server starts here.')}</h2>
            <p>
              {tr(
                'Masuk sebagai admin atau buat akun user untuk mulai membuat server Minecraft terbaik Anda.',
                'Sign in as an admin or register a user account to deploy your supreme Minecraft server today.'
              )}
            </p>
            <Link className="landing-primary mc-btn-emerald" to="/login">
              {tr('Buka MyPanel', 'Open MyPanel')}
              <ArrowUpRight />
            </Link>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="landing-footer">
        <div className="landing-wordmark">
          <span className="brand-mark mc-brand-cube">
            <span className="mc-grass-top" />
            <Server />
          </span>
          <strong>MyPanel</strong>
          <span className="landing-badge">V2</span>
        </div>
        <p>
          {tr(
            'Panel hosting Minecraft generasi berikutnya yang dibangun untuk kecepatan dan kontrol penuh.',
            'Next-generation Minecraft hosting panel engineered for pure speed and absolute control.'
          )}
        </p>
        <Link to="/login" className="footer-link">
          {tr('Masuk ke panel', 'Open panel')}
          <ChevronRight />
        </Link>
      </footer>
    </div>
  );
}
