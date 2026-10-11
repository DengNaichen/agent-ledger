import SwiftUI

struct ShellView: View {
    @State private var destination: Destination = .all
    @State private var selectedIssue: PreviewIssue?
    @State private var board = false
    @State private var search = ""
    @State private var collapsed: Set<IssueStatus> = []
    @FocusState private var searchFocused: Bool

    private var issues: [PreviewIssue] {
        PreviewIssue.samples.filter {
            destination.matches($0) && (search.isEmpty || "\($0.id) \($0.title) \($0.description)".localizedCaseInsensitiveContains(search))
        }
    }
    private var groups: [IssueStatus] {
        if case .status(let status) = destination { return [status] }
        return IssueStatus.allCases
    }

    var body: some View {
        HStack(spacing: 0) {
            sidebar
            VStack(spacing: 0) {
                header
                Rectangle().fill(Design.border).frame(height: 1)
                if let selectedIssue {
                    IssueDetailView(issue: selectedIssue)
                } else if destination == .labels {
                    labelsPage
                } else {
                    filterBar
                    Rectangle().fill(Design.border).frame(height: 1)
                    if issues.isEmpty {
                        ContentUnavailableView {
                            Label("No matching issues", systemImage: "magnifyingglass")
                        } description: {
                            Text("Try another search or choose All issues.")
                        } actions: {
                            Button("Clear filters") { search = ""; destination = .all }
                        }
                        .frame(maxWidth: .infinity, maxHeight: .infinity)
                    } else if board {
                        boardView
                    } else {
                        listView
                    }
                }
                Rectangle().fill(Design.border).frame(height: 1)
                HStack(spacing: 6) {
                    Circle().fill(Design.accent).frame(width: 5, height: 5)
                    Text("OFFLINE PREVIEW").font(Design.font(9, .semibold)).tracking(0.8)
                    Text("·  Sample data only — changes and server connection are not available.")
                        .font(Design.font(11))
                    Spacer()
                    Text("SwiftUI").font(Design.font(10))
                }
                .foregroundStyle(Design.muted)
                .padding(.horizontal, 16).frame(height: 30)
            }
            .background(Design.panel)
            .clipShape(RoundedRectangle(cornerRadius: 8))
            .overlay(RoundedRectangle(cornerRadius: 8).strokeBorder(Design.border))
            .padding(.trailing, 8).padding(.bottom, 8).padding(.top, 8)
        }
        .background(Design.background)
        .foregroundStyle(Design.text)
        .font(Design.font())
        .tint(Design.accent)
        .frame(minWidth: 980, minHeight: 640)
        .background {
            Button("Focus search") {
                selectedIssue = nil
                if destination == .labels { destination = .all }
                searchFocused = true
            }.keyboardShortcut("f", modifiers: .command).hidden()
        }
        .onExitCommand { selectedIssue = nil }
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 1) {
            HStack(spacing: 8) {
                Text("IT").font(Design.font(9, .bold)).foregroundStyle(.white)
                    .frame(width: 20, height: 20)
                    .background(Design.accent, in: RoundedRectangle(cornerRadius: 4))
                Text("InfTask").font(Design.font(13, .semibold))
                Spacer()
                Badge(title: "Preview")
            }
            .padding(.horizontal, 8).frame(height: 36).padding(.bottom, 8)

            ScrollView {
                VStack(alignment: .leading, spacing: 1) {
                    sectionLabel("Workspace")
                    navigation(.all, glyph: .list, count: PreviewIssue.samples.count)
                    navigation(.labels, glyph: .tag)
                    sectionLabel("Statuses")
                    ForEach(IssueStatus.allCases) { status in
                        navigation(.status(status), status: status, count: PreviewIssue.samples.filter { $0.status == status }.count)
                    }
                    sectionLabel("Projects")
                    navigation(.project("Mac experience"), glyph: .box)
                    navigation(.project("Delivery board"), glyph: .box)
                }
            }
            Spacer(minLength: 12)
            VStack(alignment: .leading, spacing: 6) {
                Text("A native home for your work.").foregroundStyle(Design.secondary)
                Text("Local visual prototype · v0.1").font(Design.font(11)).foregroundStyle(Design.muted)
            }
            .padding(8)
        }
        .padding(.horizontal, 8).padding(.top, 8).padding(.bottom, 12)
        .frame(width: 232)
    }

    private func sectionLabel(_ title: String) -> some View {
        Text(title).font(Design.font(12, .medium)).foregroundStyle(Design.muted)
            .padding(.horizontal, 8).padding(.top, 18).padding(.bottom, 5)
    }

    private func navigation(_ target: Destination, glyph: Glyph? = nil, status: IssueStatus? = nil, count: Int? = nil) -> some View {
        Button {
            destination = target
            selectedIssue = nil
            collapsed.removeAll()
        } label: {
            HStack(spacing: 8) {
                if let status { StatusIcon(status: status).frame(width: 16) }
                if let glyph { Icon(glyph: glyph) }
                Text(target.title).lineLimit(1)
                Spacer(minLength: 0)
                if let count { Text("\(count)").font(Design.font(12)).foregroundStyle(Design.muted) }
            }
            .foregroundStyle(destination == target ? Design.text : Design.secondary)
            .padding(.horizontal, 8).frame(height: 28)
        }
        .buttonStyle(SurfaceButton(selected: destination == target))
        .accessibilityAddTraits(destination == target ? .isSelected : [])
    }

    private var header: some View {
        HStack(spacing: 8) {
            if let selectedIssue {
                Button { self.selectedIssue = nil } label: {
                    Icon(glyph: .back).padding(5)
                }
                .buttonStyle(SurfaceButton())
                .help("Back to issues (Escape)")
                .accessibilityLabel("Back to issues")
                Text("InfTask").foregroundStyle(Design.muted)
                Text("/").foregroundStyle(Design.muted)
                Text(selectedIssue.id)
            } else {
                Text(destination.title).font(Design.font(13, .medium))
                if destination != .labels {
                    Text("\(issues.count)").font(Design.font(12)).foregroundStyle(Design.muted)
                }
            }
            Spacer()
            Text("Demo workspace").font(Design.font(11)).foregroundStyle(Design.muted)
            Text("Read only")
                .font(Design.font(11, .medium)).foregroundStyle(Design.accent)
                .padding(.horizontal, 8).padding(.vertical, 4)
                .background(Design.accent.opacity(0.08), in: RoundedRectangle(cornerRadius: 4))
        }
        .padding(.horizontal, 20).frame(height: 44)
    }

    private var filterBar: some View {
        HStack(spacing: 10) {
            HStack(spacing: 2) {
                viewButton("List", glyph: .list, isBoard: false)
                viewButton("Board", glyph: .board, isBoard: true)
            }
            .padding(2).overlay(RoundedRectangle(cornerRadius: 5).strokeBorder(Design.border))
            Spacer()
            HStack(spacing: 6) {
                Icon(glyph: .search).foregroundStyle(Design.muted)
                TextField("Search issues…", text: $search)
                    .textFieldStyle(.plain).font(Design.font(12))
                    .focused($searchFocused)
                    .accessibilityLabel("Search issues")
                if !search.isEmpty {
                    Button { search = "" } label: { Icon(glyph: .close) }
                        .buttonStyle(.plain).accessibilityLabel("Clear search")
                }
            }
            .padding(.horizontal, 8).frame(width: 210, height: 28)
            .overlay(RoundedRectangle(cornerRadius: 5).strokeBorder(searchFocused ? Design.accent : Design.border))
        }
        .padding(.horizontal, 20).frame(height: 46)
    }

    private func viewButton(_ title: String, glyph: Glyph, isBoard: Bool) -> some View {
        Button { board = isBoard } label: {
            HStack(spacing: 6) { Icon(glyph: glyph); Text(title).font(Design.font(12)) }
                .padding(.horizontal, 8).frame(height: 25)
        }
        .buttonStyle(SurfaceButton(selected: board == isBoard))
        .accessibilityAddTraits(board == isBoard ? .isSelected : [])
    }

    private var listView: some View {
        ScrollView {
            LazyVStack(spacing: 8) {
                ForEach(groups) { status in
                    let rows = issues.filter { $0.status == status }
                    VStack(spacing: 0) {
                        Button {
                            if collapsed.contains(status) { collapsed.remove(status) } else { collapsed.insert(status) }
                        } label: {
                            HStack(spacing: 10) {
                                Icon(glyph: .chevron).rotationEffect(.degrees(collapsed.contains(status) ? -90 : 0))
                                StatusIcon(status: status)
                                Text(status.rawValue).font(Design.font(13, .medium))
                                Text("\(rows.count)").foregroundStyle(Design.muted).font(Design.font(12))
                                Spacer()
                            }
                            .padding(.horizontal, 12).frame(height: 34)
                            .background(Design.raised, in: RoundedRectangle(cornerRadius: 5))
                        }
                        .buttonStyle(.plain)
                        .accessibilityLabel("\(status.rawValue), \(rows.count) issues, \(collapsed.contains(status) ? "collapsed" : "expanded")")
                        if !collapsed.contains(status) {
                            if rows.isEmpty {
                                Text("No issues").font(Design.font(12)).foregroundStyle(Design.muted)
                                    .frame(maxWidth: .infinity, alignment: .leading).padding(.leading, 64).frame(height: 42)
                            }
                            ForEach(rows) { issue in
                                Button { selectedIssue = issue } label: { issueRow(issue) }
                                    .buttonStyle(SurfaceButton())
                            }
                        }
                    }
                }
            }
            .padding(16)
        }
    }

    private func issueRow(_ issue: PreviewIssue) -> some View {
        HStack(spacing: 10) {
            PriorityIcon(priority: issue.priority).frame(width: 24)
            StatusIcon(status: issue.status).frame(width: 24)
            Text(issue.id).font(Design.font(12)).foregroundStyle(Design.muted).frame(width: 70, alignment: .leading)
            Text(issue.title).lineLimit(1).frame(maxWidth: .infinity, alignment: .leading)
            Badge(title: issue.label, dot: issue.labelColor)
            Text(issue.type).font(Design.font(11)).foregroundStyle(Design.muted).frame(width: 82, alignment: .trailing)
            Text("Oct 11").font(Design.font(11)).foregroundStyle(Design.muted).frame(width: 44, alignment: .trailing)
        }
        .padding(.horizontal, 8).frame(height: 42)
        .help("\(issue.id) — \(issue.title)")
    }

    private var boardView: some View {
        ScrollView(.horizontal) {
            HStack(alignment: .top, spacing: 16) {
                ForEach(groups) { status in
                    let rows = issues.filter { $0.status == status }
                    VStack(alignment: .leading, spacing: 12) {
                        HStack(spacing: 8) {
                            StatusIcon(status: status)
                            Text(status.rawValue).font(Design.font(13, .medium))
                            Text("\(rows.count)").foregroundStyle(Design.muted)
                            Spacer()
                        }.frame(height: 24)
                        Text(status.hint).font(Design.font(11)).foregroundStyle(Design.muted)
                        ScrollView {
                            VStack(spacing: 8) {
                                ForEach(rows) { issue in
                                    Button { selectedIssue = issue } label: {
                                        VStack(alignment: .leading, spacing: 12) {
                                            HStack {
                                                Text(issue.id).font(Design.font(11)).foregroundStyle(Design.muted)
                                                Spacer()
                                                PriorityIcon(priority: issue.priority)
                                            }
                                            Text(issue.title).font(Design.font(13, .medium)).multilineTextAlignment(.leading)
                                            HStack { Badge(title: issue.label, dot: issue.labelColor); Spacer(); StatusIcon(status: status) }
                                        }
                                        .padding(14).frame(maxWidth: .infinity, alignment: .leading)
                                        .background(Design.panel)
                                        .overlay(RoundedRectangle(cornerRadius: 6).strokeBorder(Design.border))
                                    }
                                    .buttonStyle(SurfaceButton())
                                }
                                if rows.isEmpty {
                                    Text("No issues").foregroundStyle(Design.muted).frame(maxWidth: .infinity).padding(24)
                                }
                            }
                        }
                    }
                    .padding(12).frame(width: 288)
                    .background(Design.raised, in: RoundedRectangle(cornerRadius: 8))
                }
            }
            .padding(16)
        }
    }

    private var labelsPage: some View {
        VStack(alignment: .leading, spacing: 20) {
            Text("Labels").font(Design.font(22, .semibold))
            Text("A small sample of the workspace’s shared vocabulary.").foregroundStyle(Design.muted)
            ForEach(["design", "desktop"], id: \.self) { label in
                HStack {
                    Badge(title: label, dot: label == "design" ? .purple : Design.accent)
                    Spacer()
                    Text("\(PreviewIssue.samples.filter { $0.label == label }.count) sample issues").foregroundStyle(Design.muted)
                }
                .padding(16).background(Design.raised, in: RoundedRectangle(cornerRadius: 6))
            }
            Text("Label editing is not included in this preview.").font(Design.font(12)).foregroundStyle(Design.muted)
            Spacer()
        }
        .padding(32).frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
    }
}

struct IssueDetailView: View {
    let issue: PreviewIssue

    var body: some View {
        HStack(alignment: .top, spacing: 0) {
            ScrollView {
                VStack(alignment: .leading, spacing: 24) {
                    HStack { StatusIcon(status: issue.status); Text(issue.id).foregroundStyle(Design.muted); Badge(title: issue.type) }
                    Text(issue.title).font(Design.font(28, .semibold)).tracking(-0.5).fixedSize(horizontal: false, vertical: true)
                    Text(issue.description).font(Design.font(14)).lineSpacing(7).foregroundStyle(Design.secondary)
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Preview checklist").font(Design.font(15, .semibold))
                        checklist("Compact sidebar, grouped list and board")
                        checklist("Shared light / dark colors and Inter typography")
                        checklist("Native status icons, search and detail navigation")
                    }
                    .padding(20).frame(maxWidth: .infinity, alignment: .leading)
                    .background(Design.raised, in: RoundedRectangle(cornerRadius: 8))
                    Rectangle().fill(Design.border).frame(height: 1)
                    Text("Activity").font(Design.font(14, .semibold))
                    HStack(alignment: .top, spacing: 12) {
                        Text("IT").font(Design.font(10, .semibold)).foregroundStyle(Design.accent)
                            .frame(width: 28, height: 28).background(Design.accent.opacity(0.12), in: Circle())
                        VStack(alignment: .leading, spacing: 8) {
                            HStack { Text("InfTask Preview").font(Design.font(12, .medium)); Text("Sample comment").font(Design.font(11)).foregroundStyle(Design.muted) }
                            Text("This is a visual prototype, not a live task. Nothing here reads or changes your real workspace.")
                                .foregroundStyle(Design.secondary).lineSpacing(5)
                        }
                    }
                    Text("Comments and editing will be added when a backend is connected.")
                        .font(Design.font(12)).foregroundStyle(Design.muted)
                }
                .textSelection(.enabled)
                .padding(36).frame(maxWidth: 768, alignment: .leading)
            }
            Rectangle().fill(Design.border).frame(width: 1)
            VStack(alignment: .leading, spacing: 22) {
                Text("Properties").font(Design.font(13, .semibold))
                property("Status") { HStack(spacing: 8) { StatusIcon(status: issue.status); Text(issue.status.rawValue) } }
                property("Priority") { HStack(spacing: 8) { PriorityIcon(priority: issue.priority); Text(["No priority", "Urgent", "High", "Medium", "Low"][issue.priority]) } }
                property("Project") { HStack(spacing: 8) { Icon(glyph: .box); Text(issue.project) } }
                property("Type") { Text(issue.type) }
                Rectangle().fill(Design.border).frame(height: 1)
                Text("Labels").font(Design.font(12, .medium)).foregroundStyle(Design.muted)
                Badge(title: issue.label, dot: issue.labelColor)
                Spacer()
            }
            .padding(24).frame(width: 260)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func checklist(_ text: String) -> some View {
        HStack(spacing: 10) {
            StatusIcon(status: .done)
            Text(text).font(Design.font(12)).foregroundStyle(Design.secondary)
        }
    }

    private func property<Content: View>(_ name: String, @ViewBuilder content: () -> Content) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Text(name).font(Design.font(11)).foregroundStyle(Design.muted)
            content().font(Design.font(12))
        }
    }
}
