import QtQuick
import qs.Commons

Item {
  id: root

  property real starSize: Style.space(24)
  property color iconColor: "white"
  width: starSize
  height: starSize
  implicitWidth: starSize
  implicitHeight: starSize

  Canvas {
    id: star
    anchors.fill: parent

    onIconColorChanged: requestPaint()
    onWidthChanged: requestPaint()
    onHeightChanged: requestPaint()

    property color iconColor: root.iconColor

    onPaint: {
      const ctx = getContext("2d")
      const cx = width / 2
      const cy = height / 2
      const radius = width * 0.46

      ctx.clearRect(0, 0, width, height)
      ctx.strokeStyle = iconColor
      ctx.lineWidth = Math.max(1, width * 0.05)
      ctx.lineJoin = "miter"

      for (let rotation = 0; rotation < 2; rotation++) {
        ctx.beginPath()

        for (let i = 0; i < 4; i++) {
          const angle = i * Math.PI / 2 + rotation * Math.PI / 4 - Math.PI / 2
          const x = cx + Math.cos(angle) * radius
          const y = cy + Math.sin(angle) * radius

          if (i === 0) ctx.moveTo(x, y)
          else ctx.lineTo(x, y)
        }

        ctx.closePath()
        ctx.stroke()
      }
    }
  }

  Text {
    anchors.centerIn: parent
    text: "الله"
    color: root.iconColor
    font.pixelSize: Math.max(Style.space(7), root.width * 0.28)
    font.bold: true
    font.family: "Noto Naskh Arabic"
    textFormat: Text.PlainText
    horizontalAlignment: Text.AlignHCenter
    verticalAlignment: Text.AlignVCenter
  }
}
