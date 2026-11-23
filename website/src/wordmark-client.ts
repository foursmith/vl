import { paintRow, projectWordmark, restingPose } from "./ansi-geometry"

export function enableWordmark(stage: HTMLElement) {
  const rows = stage.querySelectorAll<HTMLSpanElement>("#hero-title > span")
  let pose = { ...restingPose }
  let drag: { pointer: number; x: number; y: number } | undefined

  function draw() {
    projectWordmark(pose.yaw, pose.pitch).forEach((cells, index) => {
      rows[index].style.backgroundImage = paintRow(cells)
    })
  }

  function rotate(yaw: number, pitch: number) {
    pose = {
      yaw: (pose.yaw + yaw) % (2 * Math.PI),
      pitch: Math.min(1.45, Math.max(-1.45, pose.pitch + pitch)),
    }
    draw()
  }

  function startDrag(event: PointerEvent) {
    if (event.button !== 0 || drag) return
    stage.setPointerCapture(event.pointerId)
    drag = { pointer: event.pointerId, x: event.clientX, y: event.clientY }
  }

  function moveDrag(event: PointerEvent) {
    if (drag?.pointer !== event.pointerId) return
    const sensitivity = 3.9 / stage.clientWidth
    rotate((event.clientX - drag.x) * sensitivity, (drag.y - event.clientY) * sensitivity)
    drag.x = event.clientX
    drag.y = event.clientY
  }

  function endDrag(event: PointerEvent) {
    if (drag?.pointer === event.pointerId) drag = undefined
  }

  function rotateWithKey(event: KeyboardEvent) {
    switch (event.key) {
      case "ArrowLeft": rotate(-.16, 0); break
      case "ArrowRight": rotate(.16, 0); break
      case "ArrowUp": rotate(0, .16); break
      case "ArrowDown": rotate(0, -.16); break
      default: return
    }
    event.preventDefault()
  }

  stage.addEventListener("pointerdown", startDrag)
  stage.addEventListener("pointermove", moveDrag)
  stage.addEventListener("lostpointercapture", endDrag)
  stage.addEventListener("keydown", rotateWithKey)
}
