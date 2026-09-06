import {
  ArrowUpRight,
  Boxes,
  ChevronRight,
  CloudCog,
  FileText,
  Gauge,
  Languages,
  Menu,
  Moon,
  Server,
  ShieldCheck,
  SquareTerminal,
  Sun,
  X,
} from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useI18n } from '../i18n';
import './Landing.css';

const photos = {
  serverRoom: 'https://images.unsplash.com/photo-1762163516269-3c143e04175c?auto=format&fit=crop&w=1800&q=82',
  gamingSetup: 'https://images.unsplash.com/photo-1775410633801-5d7997f795c6?auto=format&fit=crop&w=1400&q=80',
  workstation: 'https://images.unsplash.com/photo-1754039984985-ef607d80113a?auto=format&fit=crop&w=1400&q=80',
} as const;

export function Landing() {
  const { locale, setLocale, tr } = useI18n();
  const [menuOpen, setMenuOpen] = useState(false);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => document.documentElement.dataset.theme === 'light' ? 'light' : 'dark');

  const toggleTheme = () => {
    const next = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('mypanel.theme', next);
    setTheme(next);
  };

  const closeMenu = () => setMenuOpen(false);

  return <div className="landing-page">
    <a className="landing-skip" href="#main">{tr('Lewati ke konten', 'Skip to content')}</a>
    <header className="landing-nav">
      <Link className="landing-wordmark" to="/" aria-label="MyPanel home">
        <span className="brand-mark"><Server /></span>
        <strong>MyPanel</strong>
      </Link>
      <nav className={menuOpen ? 'landing-links open' : 'landing-links'} aria-label={tr('Navigasi utama', 'Main navigation')}>
        <a href="#control" onClick={closeMenu}>{tr('Kontrol', 'Control')}</a>
        <a href="#infrastructure" onClick={closeMenu}>{tr('Infrastruktur', 'Infrastructure')}</a>
        <a href="#workflow" onClick={closeMenu}>{tr('Cara kerja', 'How it works')}</a>
        <Link className="landing-mobile-login" to="/login" onClick={closeMenu}>{tr('Masuk ke panel', 'Open panel')}<ChevronRight /></Link>
      </nav>
      <div className="landing-nav-actions">
        <button className="landing-icon-button" type="button" onClick={() => setLocale(locale === 'id' ? 'en' : 'id')} aria-label={tr('Gunakan bahasa Inggris', 'Use Indonesian')}>
          <Languages /><span>{locale.toUpperCase()}</span>
        </button>
        <button className="landing-icon-button theme" type="button" onClick={toggleTheme} aria-label={theme === 'dark' ? tr('Gunakan mode terang', 'Use light mode') : tr('Gunakan mode gelap', 'Use dark mode')}>
          {theme === 'dark' ? <Sun /> : <Moon />}
        </button>
        <Link className="landing-login" to="/login">{tr('Masuk', 'Sign in')}<ArrowUpRight /></Link>
        <button className="landing-menu-button" type="button" onClick={() => setMenuOpen((value) => !value)} aria-expanded={menuOpen} aria-label={menuOpen ? tr('Tutup menu', 'Close menu') : tr('Buka menu', 'Open menu')}>
          {menuOpen ? <X /> : <Menu />}
        </button>
      </div>
    </header>

    <main id="main">
      <section className="landing-hero">
        <div className="landing-hero-copy">
          <p className="landing-eyebrow">MINECRAFT SERVER CONTROL</p>
          <h1>{tr('Host Minecraft. Tanpa ribet.', 'Host Minecraft. Skip busywork.')}</h1>
          <p>{tr('Beli kapasitas, jalankan server, dan kelola semuanya dari satu tempat.', 'Buy capacity, launch a server, and manage everything in one place.')}</p>
          <div className="landing-hero-actions">
            <Link className="landing-primary" to="/login">{tr('Mulai sekarang', 'Get started')}<ChevronRight /></Link>
            <a className="landing-secondary" href="#workflow">{tr('Lihat cara kerja', 'See how it works')}</a>
          </div>
        </div>
        <figure className="landing-hero-media">
          <img src={photos.serverRoom} width="1800" height="1200" fetchPriority="high" alt={tr('Rak server dengan lampu status hijau', 'Server racks with green status lights')} />
          <figcaption>{tr('Infrastruktur yang siap diawasi dari satu panel.', 'Infrastructure monitored from one panel.')}</figcaption>
        </figure>
      </section>

      <section className="landing-runtime-strip" aria-label={tr('Runtime yang didukung', 'Supported runtimes')}>
        <span>{tr('Satu panel untuk', 'One panel for')}</span>
        <strong>Paper</strong><strong>Purpur</strong><strong>Fabric</strong><strong>Vanilla</strong>
      </section>

      <section className="landing-control" id="control">
        <header>
          <h2>{tr('Yang penting terlihat. Yang rumit tetap di belakang.', 'See what matters. Keep complexity behind the scenes.')}</h2>
          <p>{tr('Console, file, backup, jadwal, dan metrik memakai alur yang sama di setiap server.', 'Console, files, backups, schedules, and metrics follow the same flow on every server.')}</p>
        </header>
        <div className="landing-feature-grid">
          <article className="landing-feature feature-console">
            <SquareTerminal />
            <div><h3>{tr('Console langsung', 'Live console')}</h3><p>{tr('Log berwarna dan command muncul tanpa menunggu batch berikutnya.', 'Colored logs and commands arrive without waiting for the next batch.')}</p></div>
          </article>
          <article className="landing-feature feature-files">
            <FileText />
            <div><h3>{tr('File tanpa pindah alat', 'Files without switching tools')}</h3><p>{tr('Edit konfigurasi, unggah plugin, dan kelola folder dari browser.', 'Edit configs, upload plugins, and manage folders in the browser.')}</p></div>
          </article>
          <article className="landing-feature feature-metrics">
            <Gauge />
            <div><h3>{tr('Metrik yang mudah dibaca', 'Metrics you can read')}</h3><p>{tr('CPU, memori, dan disk mengikuti batas server yang sebenarnya.', 'CPU, memory, and disk follow the server limits that actually apply.')}</p></div>
          </article>
          <article className="landing-feature feature-visual">
            <img src={photos.workstation} width="1400" height="934" loading="lazy" alt={tr('Monitor workstation menampilkan kode dan status sistem', 'Workstation monitors showing code and system status')} />
          </article>
        </div>
      </section>

      <section className="landing-infrastructure" id="infrastructure">
        <figure>
          <img src={photos.gamingSetup} width="1400" height="933" loading="lazy" alt={tr('Meja gaming dengan beberapa monitor', 'Gaming desk with multiple monitors')} />
        </figure>
        <div>
          <CloudCog />
          <h2>{tr('Kapasitas dijaga sebelum server dibeli.', 'Capacity is checked before a server is purchased.')}</h2>
          <p>{tr('MyPanel memeriksa alokasi CPU, RAM, dan disk agar paket baru tidak melebihi kemampuan node.', 'MyPanel checks CPU, RAM, and disk allocation so new packages cannot exceed node capacity.')}</p>
          <div className="landing-infra-points">
            <span><ShieldCheck />{tr('Akses admin dan user terpisah', 'Separate admin and user access')}</span>
            <span><Boxes />{tr('Server berjalan dalam container', 'Servers run in containers')}</span>
          </div>
        </div>
      </section>

      <section className="landing-workflow" id="workflow">
        <div className="landing-workflow-copy">
          <h2>{tr('Dari paket ke dunia baru dalam satu alur.', 'From a package to a new world in one flow.')}</h2>
          <p>{tr('Tidak perlu berpindah dashboard atau menyalin konfigurasi manual.', 'No dashboard hopping or manual configuration copying required.')}</p>
        </div>
        <ol>
          <li><span>01</span><div><h3>{tr('Pilih kapasitas', 'Choose capacity')}</h3><p>{tr('Ambil paket yang sesuai dengan pemain dan plugin Anda.', 'Pick a package that fits your players and plugins.')}</p></div></li>
          <li><span>02</span><div><h3>{tr('Tentukan runtime', 'Select a runtime')}</h3><p>{tr('Pilih Paper, Purpur, Fabric, atau Vanilla beserta versinya.', 'Choose Paper, Purpur, Fabric, or Vanilla and its version.')}</p></div></li>
          <li><span>03</span><div><h3>{tr('Kelola dari panel', 'Manage from the panel')}</h3><p>{tr('Java disesuaikan otomatis, lalu console siap digunakan.', 'Java is selected automatically, then the console is ready.')}</p></div></li>
        </ol>
      </section>

      <section className="landing-cta">
        <Server />
        <h2>{tr('Server Anda berikutnya dimulai di sini.', 'Your next server starts here.')}</h2>
        <p>{tr('Masuk sebagai admin atau buat akun user dari halaman yang sama.', 'Sign in as an admin or create a user account from the same page.')}</p>
        <Link className="landing-primary" to="/login">{tr('Buka MyPanel', 'Open MyPanel')}<ArrowUpRight /></Link>
      </section>
    </main>

    <footer className="landing-footer">
      <div className="landing-wordmark"><span className="brand-mark"><Server /></span><strong>MyPanel</strong></div>
      <p>{tr('Panel hosting Minecraft yang dibangun untuk kontrol yang jelas.', 'A Minecraft hosting panel built for clear control.')}</p>
      <Link to="/login">{tr('Masuk ke panel', 'Open panel')}<ChevronRight /></Link>
    </footer>
  </div>;
}
