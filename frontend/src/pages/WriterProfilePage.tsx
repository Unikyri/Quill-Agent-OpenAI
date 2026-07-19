import WriterMemoryPanel from '../components/memory/WriterMemoryPanel'
import styles from './WriterProfilePage.module.css'

export default function WriterProfilePage() {
  return (
    <main className={styles.wrap}>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>Account</p>
        <h1>Writer profile</h1>
        <p className={styles.intro}>What Quill has learned about your writing, across every universe you have worked in.</p>
      </div>
      <WriterMemoryPanel />
    </main>
  )
}
