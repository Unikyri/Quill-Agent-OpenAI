import { useRef, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useDemoProvisioning } from '../hooks/useDemoProvisioning'
import styles from './LandingPage.module.css'

const features = [
  {
    glyph: '○',
    title: 'Living entity encyclopedia',
    text: 'Characters, places, and items with their own card, aliases, and properties — extracted automatically from your text.',
  },
  {
    glyph: '△',
    title: 'Contradiction detection',
    text: 'The AI checks every paragraph against established lore and flags anything that doesn’t add up, evidence included.',
  },
  {
    glyph: '✳',
    title: 'Relationship graph',
    text: 'Visualize how your characters and places connect. Spot links and conflicts at a glance.',
  },
  {
    glyph: '⌇',
    title: 'Consistent timeline',
    text: 'Every event ordered chronologically so a date never crosses itself.',
  },
  {
    glyph: '◠',
    title: 'Plot holes',
    text: 'Quill flags threads and characters left open and reminds you to close them before the end.',
  },
  {
    glyph: '✎',
    title: 'Editor with live analysis',
    text: 'Write at your own pace — Quill highlights entities in your text and analyzes each paragraph as you type.',
  },
]

const steps = [
  {
    n: '01',
    title: 'Bring your work',
    text: 'Write directly in the editor or upload your manuscript as .md or .txt. Quill splits it into chapters and paragraphs.',
  },
  {
    n: '02',
    title: 'Quill analyzes it',
    text: 'Extracts entities, weaves the relationship graph, orders the timeline, and detects contradictions and plot holes.',
  },
  {
    n: '03',
    title: 'You create without fear',
    text: 'Query your entire world whenever you want. Your second brain holds it all — you focus on the story.',
  },
]

export default function LandingPage() {
  const navigate = useNavigate()
  const featureCardsRef = useRef<(HTMLDivElement | null)[]>([])
  // ponytail: error surfacing skipped — no spec scenario requires it here;
  // add an inline message if a real failure case shows up in the demo.
  const { startDemo, pending: demoPending } = useDemoProvisioning()

  // ponytail: IntersectionObserver for scroll-triggered reveals (replaces GSAP ScrollTrigger)
  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            entry.target.classList.add(styles.visible)
            observer.unobserve(entry.target)
          }
        }
      },
      { threshold: 0, rootMargin: '0px 0px -80px 0px' }
    )

    featureCardsRef.current.forEach((card) => {
      if (card) observer.observe(card)
    })

    return () => observer.disconnect()
  }, [])

  const scrollTo = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
  }

  return (
    <div className={styles.page}>
      {/* Navbar */}
      <header className={styles.navbar}>
        <span className={styles.brandMark}>Q</span>
        <span className={styles.logo}>Quill</span>
        <nav className={styles.navLinks}>
          <button className={styles.navLink} onClick={() => scrollTo('features')}>
            What it does
          </button>
          <button className={styles.navLink} onClick={() => scrollTo('how')}>
            How it works
          </button>
          <button className={styles.navCta} onClick={() => navigate('/login')}>
            Log in
          </button>
        </nav>
      </header>

      {/* Hero */}
      <section className={styles.hero}>
        <svg
          viewBox="0 0 1400 620"
          preserveAspectRatio="xMidYMid slice"
          className={styles.heroSvg}
          aria-hidden
        >
          <g fill="none" stroke="rgba(233,225,207,.08)" strokeWidth="1.2">
            <path d="M-40 140 Q300 90 700 160 T1440 140" />
            <path d="M-40 260 Q300 210 700 280 T1440 260" />
            <path d="M-40 400 Q320 350 720 420 T1460 400" />
            <path d="M-40 520 Q300 470 700 540 T1440 520" />
          </g>
          <g stroke="rgba(233,225,207,.22)" strokeWidth="1.3">
            <line x1="1060" y1="200" x2="1180" y2="320" />
            <line x1="1180" y1="320" x2="1300" y2="240" />
            <line x1="1180" y1="320" x2="1120" y2="460" />
            <line x1="1180" y1="320" x2="980" y2="400" />
          </g>
          <line
            x1="980"
            y1="400"
            x2="1300"
            y2="240"
            stroke="#d98b63"
            strokeWidth="1.6"
            strokeDasharray="6 6"
            className={styles.dash}
          />
          <g className={styles.float1}>
            <circle cx="1180" cy="320" r="34" fill="#d9a441" />
            <circle
              cx="1180"
              cy="320"
              r="34"
              fill="none"
              stroke="rgba(217,164,65,.3)"
              strokeWidth="10"
              className={styles.glow}
            />
          </g>
          <g className={styles.float2}>
            <circle cx="1060" cy="200" r="20" fill="#e9e1cf" />
          </g>
          <g className={styles.float3}>
            <circle cx="1300" cy="240" r="20" fill="#8fae86" />
          </g>
          <g className={styles.float4}>
            <circle cx="1120" cy="460" r="17" fill="#c98a5c" />
          </g>
          <g className={styles.float5}>
            <circle cx="980" cy="400" r="17" fill="#e9e1cf" />
          </g>
        </svg>
        <div className={styles.heroGradient} />
        <div className={styles.heroInner}>
          <p className={styles.heroTagline}>The second brain for worldbuilders</p>
          <h1 className={styles.heroTitle}>
            Your world grows. Your memory doesn&rsquo;t have to.
          </h1>
          <p className={styles.heroSubtitle}>
            Quill reads your novels, scripts, and manga, extracts every character, place,
            and lore rule, and watches for contradictions while you write. For writers,
            screenwriters, and mangaka who never want to forget anything again.
          </p>
          <div className={styles.heroCtas}>
            <button className={styles.heroCta} onClick={() => void startDemo()} disabled={demoPending}>
              {demoPending ? 'Setting up your demo…' : 'Try the live demo'}
            </button>
            <button className={styles.heroCtaGhost} onClick={() => scrollTo('how')}>
              See how it works
            </button>
          </div>
        </div>
      </section>

      {/* Problem statement */}
      <section className={styles.problem}>
        <h2 className={styles.problemTitle}>Forgetting one detail breaks a whole world.</h2>
        <p className={styles.problemText}>
          Which chapter did that character die in? What color were their eyes? Was that
          object made of gold or obsidian? When a work grows across hundreds of pages,
          keeping it all consistent by hand becomes impossible. Quill remembers it for you.
        </p>
      </section>

      {/* Features */}
      <section className={styles.features} id="features">
        <div className={styles.featureGrid}>
          {features.map((f, i) => (
            <div
              key={f.title}
              className={styles.featureCard}
              ref={(el) => { featureCardsRef.current[i] = el }}
            >
              <div className={`${styles.featureIcon} glyph`} aria-hidden>
                {f.glyph}
              </div>
              <h3 className={styles.featureCardTitle}>{f.title}</h3>
              <p className={styles.featureCardText}>{f.text}</p>
            </div>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section className={styles.how} id="how">
        <div className={styles.howHeader}>
          <p className={styles.howLabel}>How it works</p>
          <h2 className={styles.howTitle}>You write. Quill remembers.</h2>
        </div>
        <div className={styles.howGrid}>
          {steps.map((s) => (
            <div key={s.n} className={styles.howStep}>
              <div className={styles.howNumeral}>{s.n}</div>
              <h3 className={styles.howStepTitle}>{s.title}</h3>
              <p className={styles.howStepText}>{s.text}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Closing CTA */}
      <section className={styles.closingWrap}>
        <div className={styles.closing}>
          <h2 className={styles.closingTitle}>Stop holding your world in your head.</h2>
          <p className={styles.closingText}>
            Start building your universe with Quill and write knowing nothing gets lost.
          </p>
          <button className={styles.closingCta} onClick={() => navigate('/login')}>
            Enter Quill
          </button>
        </div>
      </section>

      {/* Footer */}
      <footer className={styles.footer}>
        <span className={styles.footerMark}>Q</span>
        <span className={styles.footerLogo}>Quill</span>
        <span className={styles.footerText}>A second brain for worldbuilders.</span>
      </footer>
    </div>
  )
}
