import QtQuick
import qs.Commons
import qs.Ui

// REUSABLE: searchable picker page for full-panel list selection.
// Consumers provide generic field accessors (valueOf/titleOf/...)
// and react to the selected/closeRequested/searchEdited signals.
BorderSurface {
  id: picker

  color: "transparent"
  radius: 0
  borderSpec: Border.none()

  property string title: ""
  property string countSuffix: "available"
  property string placeholder: ""
  property string closeName: "Close picker"
  property var items: []
  property string selectedValue: ""
  property var valueOf: function(modelData) { return String(modelData) }
  property var titleOf: function(modelData) { return String(modelData) }
  property var secondaryOf: function(modelData) { return "" }
  property var tagOf: function(modelData) { return "" }
  property real rowHeight: Style.space(52)
  property string error: ""
  property bool hasCachedItems: false
  property bool loading: false
  property string loadingLabel: "Loading…"
  property string emptyText: ""
  property bool open: false
  property color foreground: Color.foreground
  property string fontFamily: "monospace"

  // INFO: read-only view of the search field text
  readonly property alias searchQuery: searchField.text

  signal selected(string value)
  signal closeRequested()
  signal searchEdited(string text)

  onOpenChanged: {
    if (open) {
      // NOTE: assignment pattern avoids relying on a possibly broken text binding
      searchField.text = ""
      picker.searchEdited("")
      Qt.callLater(function() {
        searchField.forceActiveFocus()
      })
    }
  }

  function selectIndex(index) {
    if (index >= 0 && index < items.length)
      picker.selected(picker.valueOf(items[index]))
  }

  // NOTE: page layout, so the back button sits in the header instead of a card edge
  Item {
    anchors.fill: parent

    Row {
      id: headerRow

      anchors.top: parent.top
      anchors.left: parent.left
      anchors.right: parent.right
      height: Style.space(32)
      spacing: Style.space(8)

      Button {
        id: backButton

        width: Style.space(32)
        height: Style.space(32)
        anchors.verticalCenter: parent.verticalCenter
        iconText: "󰁍"
        iconSize: Style.font.icon
        tooltipText: "Back (Esc)"
        Accessible.name: picker.closeName
        onClicked: picker.closeRequested()
      }

      Column {
        width: parent.width - backButton.width - parent.spacing
        anchors.verticalCenter: parent.verticalCenter
        spacing: Style.space(2)

        Text {
          width: parent.width
          text: picker.title
          color: picker.foreground
          font.family: picker.fontFamily
          font.pixelSize: Style.font.heading
          font.bold: true
          elide: Text.ElideRight
        }

        Text {
          width: parent.width
          text: picker.items.length + " " + picker.countSuffix
          color: Qt.darker(picker.foreground, 1.4)
          font.family: picker.fontFamily
          font.pixelSize: Style.font.caption
        }
      }
    }

    TextField {
      id: searchField

      anchors.top: headerRow.bottom
      anchors.topMargin: Style.space(10)
      anchors.left: parent.left
      anchors.right: parent.right
      height: Style.spacing.controlHeight
      placeholderText: picker.placeholder
      foreground: picker.foreground
      accent: Color.accent
      font.family: picker.fontFamily
      font.pixelSize: Style.font.body
      selectByMouse: true

      onTextChanged: picker.searchEdited(text)

      Keys.onPressed: function(event) {
        if (event.key === Qt.Key_Escape) {
          picker.closeRequested()
          event.accepted = true
        } else if (event.key === Qt.Key_Down) {
          if (listView.count > 0) {
            listView.currentIndex = 0
            listView.forceActiveFocus()
          }
          event.accepted = true
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
          if (listView.count > 0 && listView.currentIndex >= 0)
            picker.selectIndex(listView.currentIndex)
          event.accepted = true
        }
      }
    }

    PanelSeparator {
      id: separator

      anchors.top: searchField.bottom
      anchors.topMargin: Style.space(10)
      anchors.left: parent.left
      anchors.right: parent.right
      foreground: picker.foreground
      strength: 0.1
    }

    BorderSurface {
      id: errorBanner

      visible: picker.error !== ""
      anchors.top: separator.bottom
      anchors.topMargin: Style.space(6)
      anchors.left: parent.left
      anchors.right: parent.right
      radius: Style.cornerRadius
      color: Qt.rgba(1, 0.2, 0.2, 0.15)
      borderSpec: Border.localOrSurfaceSpec("popups", "border", "#ff6b6b", "#ff6b6b", 1)
      implicitHeight: errorText.implicitHeight + Style.space(12)

      Text {
        id: errorText

        anchors.centerIn: parent
        text: picker.error + (picker.hasCachedItems ? " (showing cached)" : "")
        color: "#ff6b6b"
        font.family: picker.fontFamily
        font.pixelSize: Style.font.caption
      }
    }

    Column {
      anchors.centerIn: parent
      spacing: Style.space(8)
      visible: picker.loading

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "󰑐"
        font.family: picker.fontFamily
        font.pixelSize: Style.space(28)
        color: Color.accent
      }

      Text {
        anchors.horizontalCenter: parent.horizontalCenter
        text: picker.loadingLabel
        font.family: picker.fontFamily
        font.pixelSize: Style.font.bodySmall
        color: Qt.darker(picker.foreground, 1.3)
      }
    }

    Text {
      anchors.centerIn: parent
      visible: !picker.loading && picker.items.length === 0 && picker.error === ""
      text: picker.emptyText
      font.family: picker.fontFamily
      font.pixelSize: Style.font.bodySmall
      color: Qt.darker(picker.foreground, 1.4)
      wrapMode: Text.WordWrap
      horizontalAlignment: Text.AlignHCenter
      width: parent.width - Style.space(32)
    }

    ListView {
      id: listView

      anchors.top: errorBanner.visible ? errorBanner.bottom : separator.bottom
      anchors.topMargin: errorBanner.visible ? Style.space(6) : Style.space(10)
      anchors.left: parent.left
      anchors.right: parent.right
      anchors.bottom: parent.bottom
      clip: true
      spacing: Style.space(4)
      boundsBehavior: Flickable.StopAtBounds
      model: picker.items

      Keys.priority: Keys.BeforeItem
      Keys.onPressed: function(event) {
        if (event.key === Qt.Key_Escape) {
          picker.closeRequested()
          event.accepted = true
        } else if (event.key === Qt.Key_Down || event.text === "j") {
          if (currentIndex < count - 1)
            currentIndex++
          event.accepted = true
        } else if (event.key === Qt.Key_Up || event.text === "k") {
          if (currentIndex <= 0) {
            searchField.forceActiveFocus()
          } else {
            currentIndex--
          }
          event.accepted = true
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter) {
          picker.selectIndex(currentIndex)
          event.accepted = true
        }
      }

      delegate: BorderSurface {
        id: rowItem

        required property var modelData
        required property int index

        readonly property bool isSelected: String(picker.valueOf(modelData)) === picker.selectedValue
        readonly property bool isCurrent: index === listView.currentIndex

        width: listView.width
        height: picker.rowHeight
        radius: Style.cornerRadius
        color: isSelected ? Style.selectedFillFor(picker.foreground, Color.accent) : (isCurrent || rowMouse.containsMouse ? Style.hoverFillFor(picker.foreground, Color.accent) : "transparent")
        borderSpec: isSelected ? Border.controlSpec("selected", picker.foreground, Color.accent) : (isCurrent ? Border.controlSpec("focus", picker.foreground, Color.accent) : Border.none())

        Row {
          anchors.fill: parent
          anchors.leftMargin: Style.space(10)
          anchors.rightMargin: Style.space(10)
          spacing: Style.space(8)

          Column {
            width: parent.width - badgeRow.width - parent.spacing
            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(2)

            Text {
              width: parent.width
              text: picker.titleOf(rowItem.modelData)
              color: picker.foreground
              font.family: picker.fontFamily
              font.pixelSize: Style.font.bodySmall
              font.bold: rowItem.isSelected
              elide: Text.ElideRight
            }

            Text {
              width: parent.width
              text: picker.secondaryOf(rowItem.modelData)
              color: Qt.darker(picker.foreground, 1.45)
              font.family: picker.fontFamily
              font.pixelSize: Style.font.caption
              elide: Text.ElideRight
            }
          }

          Row {
            id: badgeRow

            anchors.verticalCenter: parent.verticalCenter
            spacing: Style.space(6)

            BorderSurface {
              implicitWidth: tagText.implicitWidth + Style.space(10)
              implicitHeight: tagText.implicitHeight + Style.space(6)
              anchors.verticalCenter: parent.verticalCenter
              radius: Style.space(4)
              color: Qt.rgba(picker.foreground.r, picker.foreground.g, picker.foreground.b, 0.08)
              borderSpec: Border.none()

              Text {
                id: tagText

                anchors.centerIn: parent
                text: picker.tagOf(rowItem.modelData)
                color: Qt.darker(picker.foreground, 1.3)
                font.family: picker.fontFamily
                font.pixelSize: Style.space(9)
                font.bold: true
              }
            }

            Text {
              visible: rowItem.isSelected
              anchors.verticalCenter: parent.verticalCenter
              text: "󰄬"
              color: Color.accent
              font.family: picker.fontFamily
              font.pixelSize: Style.font.icon
            }
          }
        }

        MouseArea {
          id: rowMouse

          anchors.fill: parent
          hoverEnabled: true
          cursorShape: Qt.PointingHandCursor
          onPositionChanged: listView.currentIndex = rowItem.index
          onClicked: picker.selectIndex(rowItem.index)
        }
      }
    }
  }
}
