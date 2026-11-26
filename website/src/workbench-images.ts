import browseDark from "./assets/browse-dark.webp?url"
import browseLight from "./assets/browse-light.webp?url"
import diffDark from "./assets/diff-dark.webp?url"
import diffLight from "./assets/diff-light.webp?url"
import editDark from "./assets/edit-dark.webp?url"
import editLight from "./assets/edit-light.webp?url"
import browseDarkSmall from "./assets/browse-dark-840.webp?url"
import browseLightSmall from "./assets/browse-light-840.webp?url"
import diffDarkSmall from "./assets/diff-dark-840.webp?url"
import diffLightSmall from "./assets/diff-light-840.webp?url"
import editDarkSmall from "./assets/edit-dark-840.webp?url"
import editLightSmall from "./assets/edit-light-840.webp?url"

export const screenshots = [
  { Dark: { src: browseDark, small: browseDarkSmall, height: 1001 }, Light: { src: browseLight, small: browseLightSmall } },
  { Dark: { src: diffDark, small: diffDarkSmall, height: 1003 }, Light: { src: diffLight, small: diffLightSmall } },
  { Dark: { src: editDark, small: editDarkSmall, height: 1003 }, Light: { src: editLight, small: editLightSmall } },
] as const

export const screenshotSizes = "(min-width: 72rem) 768px, (min-width: 64rem) 66.67vw, 100vw"

export function screenshotSrcset(image: { src: string; small: string }) {
  return `${image.small} 840w, ${image.src} 1440w`
}
