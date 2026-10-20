import type { LegalSection } from '../model/content'
import styles from './InfoPage.module.scss'

// документ (условия, политика): нумерованные разделы из абзацев и списков
export function LegalDocument({ sections }: { sections: LegalSection[] }) {
    return (
        <ol className={styles.sections}>
            {sections.map((section, index) => (
                <li key={section.title} className={styles.legal}>
                    <h2 className={styles.legalTitle}>
                        {index + 1}. {section.title}
                    </h2>
                    {section.paragraphs?.map((paragraph) => (
                        <p key={paragraph} className={styles.paragraph}>
                            {paragraph}
                        </p>
                    ))}
                    {section.list && (
                        <ul className={styles.list}>
                            {section.list.map((item) => (
                                <li key={item}>{item}</li>
                            ))}
                        </ul>
                    )}
                </li>
            ))}
        </ol>
    )
}
