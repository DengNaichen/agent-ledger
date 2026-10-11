import SwiftUI

// Native equivalents of web/src/app.css. Keep light/dark pairs in one place.
enum Design {
    static func color(_ light: UInt32, _ dark: UInt32) -> Color {
        Color(nsColor: NSColor(name: nil) { appearance in
            let hex = appearance.bestMatch(from: [.aqua, .darkAqua]) == .darkAqua ? dark : light
            return NSColor(srgbRed: CGFloat((hex >> 16) & 255) / 255,
                           green: CGFloat((hex >> 8) & 255) / 255,
                           blue: CGFloat(hex & 255) / 255, alpha: 1)
        })
    }

    static let background = color(0xf5f6f8, 0x08090a)
    static let panel = color(0xffffff, 0x0f1011)
    static let raised = color(0xf0f1f4, 0x141516)
    static let text = color(0x202127, 0xf7f8f8)
    static let secondary = color(0x535963, 0x9ca1a9)
    static let muted = color(0x606773, 0x80858d)
    static let border = color(0xe3e4e7, 0x222324)
    static let selected = color(0xe7e8eb, 0x1d1e1f)
    static let accent = color(0x5661c4, 0x5e6ad2)
    static let hover = color(0xedeef0, 0x17181a)
    static func font(_ size: CGFloat = 13, _ weight: Font.Weight = .regular) -> Font {
        .custom("InterVariable", size: size).weight(weight)
    }
}

struct SurfaceButton: ButtonStyle {
    var selected = false
    @State private var hovering = false

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .contentShape(Rectangle())
            .background(selected ? Design.selected : (hovering || configuration.isPressed ? Design.hover : .clear))
            .clipShape(RoundedRectangle(cornerRadius: 5))
            .onHover { hovering = $0 }
    }
}

struct Badge: View {
    let title: String
    var dot: Color? = nil

    var body: some View {
        HStack(spacing: 5) {
            if let dot { Circle().fill(dot).frame(width: 6, height: 6) }
            Text(title).lineLimit(1)
        }
        .font(Design.font(11))
        .foregroundStyle(Design.secondary)
        .padding(.horizontal, 7).padding(.vertical, 3)
        .background(Design.raised, in: RoundedRectangle(cornerRadius: 4))
    }
}

// Lucide geometry, rendered as native paths, not SF Symbols. See bundled ISC license.
enum Glyph { case list, board, box, search, chevron, back, tag, close }

struct Icon: View {
    let glyph: Glyph
    var body: some View {
        Canvas { context, size in
            context.scaleBy(x: size.width / 24, y: size.height / 24)
            var path = Path()
            func line(_ points: [CGPoint]) {
                path.move(to: points[0])
                points.dropFirst().forEach { path.addLine(to: $0) }
            }
            switch glyph {
            case .list:
                for y in [6.0, 12, 18] {
                    line([CGPoint(x: 8, y: y), CGPoint(x: 21, y: y)])
                    line([CGPoint(x: 3, y: y), CGPoint(x: 3.01, y: y)])
                }
            case .board:
                path.addRoundedRect(in: CGRect(x: 3, y: 3, width: 18, height: 18), cornerSize: CGSize(width: 2, height: 2))
                line([CGPoint(x: 12, y: 3), CGPoint(x: 12, y: 21)])
            case .box:
                line([CGPoint(x: 12, y: 3), CGPoint(x: 21, y: 8), CGPoint(x: 21, y: 17), CGPoint(x: 12, y: 22), CGPoint(x: 3, y: 17), CGPoint(x: 3, y: 8), CGPoint(x: 12, y: 3)])
                line([CGPoint(x: 3, y: 8), CGPoint(x: 12, y: 13), CGPoint(x: 21, y: 8)])
                line([CGPoint(x: 12, y: 13), CGPoint(x: 12, y: 22)])
            case .search:
                path.addEllipse(in: CGRect(x: 3, y: 3, width: 16, height: 16))
                line([CGPoint(x: 16.65, y: 16.65), CGPoint(x: 21, y: 21)])
            case .chevron:
                line([CGPoint(x: 6, y: 9), CGPoint(x: 12, y: 15), CGPoint(x: 18, y: 9)])
            case .back:
                line([CGPoint(x: 12, y: 19), CGPoint(x: 5, y: 12), CGPoint(x: 12, y: 5)])
                line([CGPoint(x: 5, y: 12), CGPoint(x: 19, y: 12)])
            case .tag:
                line([CGPoint(x: 3, y: 3), CGPoint(x: 12, y: 3), CGPoint(x: 22, y: 13), CGPoint(x: 13, y: 22), CGPoint(x: 3, y: 12), CGPoint(x: 3, y: 3)])
                path.addEllipse(in: CGRect(x: 7, y: 7, width: 1, height: 1))
            case .close:
                line([CGPoint(x: 6, y: 6), CGPoint(x: 18, y: 18)])
                line([CGPoint(x: 18, y: 6), CGPoint(x: 6, y: 18)])
            }
            context.stroke(path, with: .foreground, style: StrokeStyle(lineWidth: 2.25, lineCap: .round, lineJoin: .round))
        }
        .frame(width: 16, height: 16)
        .accessibilityHidden(true)
    }
}

struct StatusIcon: View {
    let status: IssueStatus
    var body: some View {
        Canvas { context, _ in
            let ring = Path(ellipseIn: CGRect(x: 1, y: 1, width: 12, height: 12))
            let color = GraphicsContext.Shading.color(status.color)
            if [.triage, .done, .canceled].contains(status) {
                context.fill(Path(ellipseIn: CGRect(x: 0, y: 0, width: 14, height: 14)), with: color)
                var mark = Path()
                let segments: [[CGPoint]]
                switch status {
                case .done:
                    segments = [[CGPoint(x: 4.25, y: 7.25), CGPoint(x: 6, y: 9), CGPoint(x: 9.75, y: 5)]]
                case .canceled:
                    segments = [[CGPoint(x: 5, y: 5), CGPoint(x: 9, y: 9)], [CGPoint(x: 9, y: 5), CGPoint(x: 5, y: 9)]]
                default:
                    segments = [[CGPoint(x: 3.5, y: 5), CGPoint(x: 10.5, y: 5)], [CGPoint(x: 8.5, y: 3), CGPoint(x: 10.5, y: 5), CGPoint(x: 8.5, y: 7)], [CGPoint(x: 10.5, y: 9), CGPoint(x: 3.5, y: 9)], [CGPoint(x: 5.5, y: 7), CGPoint(x: 3.5, y: 9), CGPoint(x: 5.5, y: 11)]]
                }
                for segment in segments {
                    mark.move(to: segment[0])
                    segment.dropFirst().forEach { mark.addLine(to: $0) }
                }
                context.stroke(mark, with: .color(Design.panel), style: StrokeStyle(lineWidth: 1.25, lineCap: .round, lineJoin: .round))
            } else {
                context.stroke(ring, with: color, style: StrokeStyle(lineWidth: 1.5, dash: status == .backlog ? [2.36, 2.36] : []))
                if status == .agentReady {
                    context.fill(Path(ellipseIn: CGRect(x: 5, y: 5, width: 4, height: 4)), with: color)
                }
                if status == .inProgress || status == .inReview {
                    var pie = Path()
                    pie.move(to: CGPoint(x: 7, y: 7))
                    pie.addArc(center: CGPoint(x: 7, y: 7), radius: 3.5, startAngle: .degrees(-90), endAngle: .degrees(status == .inProgress ? 90 : 180), clockwise: false)
                    pie.closeSubpath()
                    context.fill(pie, with: color)
                }
            }
        }
        .frame(width: 14, height: 14)
        .accessibilityLabel(status.rawValue)
    }
}

struct PriorityIcon: View {
    let priority: Int
    var body: some View {
        Canvas { context, _ in
            for bar in 0..<3 {
                let rect = CGRect(x: 2 + Double(bar) * 4.5, y: priority == 0 ? 7 : Double(9 - bar * 3), width: 3, height: priority == 0 ? 1.5 : Double(5 + bar * 3))
                let active = priority == 0 || bar < 5 - priority
                context.fill(Path(roundedRect: rect, cornerRadius: 1), with: .color(Design.muted.opacity(active ? 1 : 0.25)))
            }
        }
        .frame(width: 16, height: 16)
        .accessibilityLabel(priority == 0 ? "No priority" : "Priority \(priority)")
    }
}
