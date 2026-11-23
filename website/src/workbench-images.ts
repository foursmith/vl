import browseDark from "./assets/browse-dark.webp?url"
import browseLight from "./assets/browse-light.webp?url"
import diffDark from "./assets/diff-dark.webp?url"
import diffLight from "./assets/diff-light.webp?url"
import historyDark from "./assets/history-dark.webp?url"
import historyLight from "./assets/history-light.webp?url"
import browseDarkSmall from "./assets/browse-dark-840.webp?url"
import browseLightSmall from "./assets/browse-light-840.webp?url"
import diffDarkSmall from "./assets/diff-dark-840.webp?url"
import diffLightSmall from "./assets/diff-light-840.webp?url"
import historyDarkSmall from "./assets/history-dark-840.webp?url"
import historyLightSmall from "./assets/history-light-840.webp?url"

export const screenshots = [
  { Dark: { src: browseDark, small: browseDarkSmall }, Light: { src: browseLight, small: browseLightSmall } },
  { Dark: { src: diffDark, small: diffDarkSmall }, Light: { src: diffLight, small: diffLightSmall } },
  { Dark: { src: historyDark, small: historyDarkSmall }, Light: { src: historyLight, small: historyLightSmall } },
] as const

export const screenshotSizes = "(min-width: 1152px) 1152px, 100vw"

export function screenshotSrcset(image: { src: string; small: string }) {
  return `${image.small} 840w, ${image.src} 1200w`
}

export const themes = ["Dark", "Light"] as const
