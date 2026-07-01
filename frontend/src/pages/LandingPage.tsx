import { useRef, useLayoutEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import gsap from 'gsap'
import { ScrollTrigger } from 'gsap/ScrollTrigger'
import styles from './LandingPage.module.css'

gsap.registerPlugin(ScrollTrigger)

const features = [
  {
    title: 'Build Worlds',
    text: 'Create rich universes with characters, locations, and lore. Every detail tracked so nothing falls through the cracks.',
  },
  {
    title: 'Write with AI',
    text: 'Qwen-powered intelligence helps you navigate contradictions, fill plot holes, and keep your timeline consistent.',
  },
  {
    title: 'See Connections',
    text: 'An interactive knowledge graph reveals relationships between every entity, event, and chapter in your universe.',
  },
  {
    title: 'Stay Organized',
    text: 'Works, chapters, drafts — all structured like a manuscript. Focus on writing, not on managing files.',
  },
]

export default function LandingPage() {
  const navigate = useNavigate()
  const heroRef = useRef<HTMLElement>(null)
  const featureCardsRef = useRef<(HTMLDivElement | null)[]>([])
  const closingRef = useRef<HTMLElement>(null)

  useLayoutEffect(() => {
    const ctx = gsap.context(() => {
      // Hero — staggered reveal on load (no scroll needed for first impression)
      const hero = heroRef.current
      if (hero) {
        const tl = gsap.timeline({ defaults: { ease: 'power2.out' } })
        tl.from(hero.querySelector('[data-anim="tagline"]'), { opacity: 0, y: 16, duration: 0.7 })
          .from(hero.querySelector('[data-anim="title"]'), { opacity: 0, y: 24, duration: 0.8 }, '-=0.3')
          .from(hero.querySelector('[data-anim="subtitle"]'), { opacity: 0, y: 16, duration: 0.7 }, '-=0.3')
          .from(hero.querySelector('[data-anim="cta"]'), { opacity: 0, y: 12, duration: 0.6 }, '-=0.2')
          .from(hero.querySelector('[data-anim="ornament"]'), { opacity: 0, scaleX: 0, duration: 0.6 }, '-=0.1')
      }

      // Feature cards — scroll-triggered fade+reveal
      featureCardsRef.current.forEach((card, i) => {
        if (!card) return
        gsap.from(card, {
          scrollTrigger: {
            trigger: card,
            start: 'top bottom-=80',
            toggleActions: 'play none none none',
          },
          opacity: 0,
          y: 32,
          duration: 0.7,
          delay: i * 0.12,
          ease: 'power2.out',
        })
      })

      // Closing section — scroll-triggered
      if (closingRef.current) {
        gsap.from(closingRef.current.querySelector('[data-anim="quote"]'), {
          scrollTrigger: {
            trigger: closingRef.current,
            start: 'top bottom-=60',
            toggleActions: 'play none none none',
          },
          opacity: 0,
          y: 24,
          duration: 0.8,
          ease: 'power2.out',
        })
        gsap.from(closingRef.current.querySelector('[data-anim="author"]'), {
          scrollTrigger: {
            trigger: closingRef.current,
            start: 'top bottom-=40',
            toggleActions: 'play none none none',
          },
          opacity: 0,
          y: 16,
          duration: 0.6,
          delay: 0.2,
          ease: 'power2.out',
        })
      }
    })

    return () => ctx.revert()
  }, [])

  const scrollTo = (id: string) => {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' })
  }

  return (
    <div className={styles.page}>
      {/* Navbar */}
      <nav className={styles.navbar}>
        <span className={styles.logo}>Quill</span>
        <div className={styles.navLinks}>
          <button className={styles.navLink} onClick={() => scrollTo('features')}>
            Features
          </button>
          <button className={styles.navLink} onClick={() => scrollTo('closing')}>
            About
          </button>
          <button className={styles.navCta} onClick={() => navigate('/login')}>
            Start Writing
          </button>
        </div>
      </nav>

      {/* Hero */}
      <section className={styles.hero} ref={heroRef}>
        <p className={styles.heroTagline} data-anim="tagline">
          For writers who build worlds
        </p>
        <h1 className={styles.heroTitle} data-anim="title">
          Write universes,
          <br />
          not just stories.
        </h1>
        <p className={styles.heroSubtitle} data-anim="subtitle">
          Quill is an AI-powered writing IDE that helps you craft rich, consistent
          fiction — from the first character sketch to the final chapter.
        </p>
        <button
          className={styles.heroCta}
          data-anim="cta"
          onClick={() => navigate('/login')}
        >
          Try the Demo
        </button>
        <div className={styles.heroOrnament} data-anim="ornament" />
      </section>

      {/* Features */}
      <section className={styles.features} id="features">
        <p className={styles.featuresLabel}>What Quill offers</p>
        <h2 className={styles.featuresTitle}>Everything a writer needs</h2>
        <div className={styles.featureGrid}>
          {features.map((f, i) => (
            <div
              key={f.title}
              className={styles.featureCard}
              ref={(el) => { featureCardsRef.current[i] = el }}
            >
              <div className={styles.featureIcon} aria-hidden>
                {['🖋️', '🤖', '🔗', '📜'][i]}
              </div>
              <h3 className={styles.featureCardTitle}>{f.title}</h3>
              <p className={styles.featureCardText}>{f.text}</p>
            </div>
          ))}
        </div>
      </section>

      {/* Closing */}
      <section className={styles.closing} id="closing" ref={closingRef}>
        <p className={styles.closingQuote} data-anim="quote">
          "A writer is a world trapped in a person."
        </p>
        <p className={styles.closingAuthor} data-anim="author">— Victor Hugo</p>
        <button className={styles.closingCta} onClick={() => navigate('/login')}>
          Start Your Universe
        </button>
      </section>

      {/* Footer */}
      <footer className={styles.footer}>
        <p className={styles.footerText}>Quill — Crafted with ink and intelligence</p>
      </footer>
    </div>
  )
}
