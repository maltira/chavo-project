import { Icon } from '@/shared/ui'

import { FAQ } from '../model/content'
import styles from './InfoPage.module.scss'

// вопросы по темам: тема - плитка с иконкой и заголовок, вопрос раскрывается по нажатию
export function FaqList() {
    return (
        <div className={styles.sections}>
            {FAQ.map((group) => (
                <section key={group.title} className={styles.group}>
                    <h2 className={styles.groupTitle}>
                        <span className={styles.tile}>
                            <Icon name={group.icon} size={22} color="inherit" />
                        </span>
                        {group.title}
                    </h2>
                    <div className={styles.questions}>
                        {group.items.map((item) => (
                            <details key={item.question} className={styles.question}>
                                <summary className={styles.summary}>
                                    <span>{item.question}</span>
                                    <Icon
                                        name="arrow-right"
                                        size={20}
                                        color="inherit"
                                        className={styles.chevron}
                                    />
                                </summary>
                                <div className={styles.answer}>
                                    {item.answer.map((paragraph) => (
                                        <p key={paragraph}>{paragraph}</p>
                                    ))}
                                </div>
                            </details>
                        ))}
                    </div>
                </section>
            ))}
        </div>
    )
}
