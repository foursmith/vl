import browseDark from "./assets/browse-dark.webp?url"
import browseLight from "./assets/browse-light.webp?url"
import diffDark from "./assets/diff-dark.webp?url"
import diffLight from "./assets/diff-light.webp?url"
import editDark from "./assets/edit-dark.webp?url"
import editLight from "./assets/edit-light.webp?url"

export const screenshots = [
  { dark: browseDark, light: browseLight, darkHeight: 1001 },
  { dark: diffDark, light: diffLight, darkHeight: 1003 },
  { dark: editDark, light: editLight, darkHeight: 1003 },
] as const
