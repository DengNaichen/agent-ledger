import SwiftUI

enum IssueStatus: String, CaseIterable, Identifiable {
    case triage = "Triage", backlog = "Backlog", todo = "Todo", agentReady = "Agent ready"
    case inProgress = "In progress", inReview = "In review", done = "Done", canceled = "Canceled"
    var id: String { rawValue }
    var color: Color {
        switch self {
        case .triage: Design.color(0xfc7840, 0xfc7840)
        case .backlog: Design.color(0x737b86, 0xbec2c8)
        case .todo: Design.color(0x626974, 0xe2e2e2)
        case .agentReady: Design.color(0x8665b0, 0xb49bdd)
        case .inProgress: Design.color(0x987000, 0xf2c94c)
        case .inReview: Design.color(0x258253, 0x4cb782)
        case .done: Design.color(0x5e6ad2, 0x5e6ad2)
        case .canceled: Design.color(0x697789, 0x95a2b3)
        }
    }
    var hint: String {
        switch self {
        case .triage: "Worth keeping?"
        case .backlog: "Not planned yet"
        case .todo: "Planned work"
        case .agentReady: "Clear enough to start"
        case .inProgress: "One active commitment"
        case .inReview: "Prove it works"
        case .done: "Merged and verified"
        case .canceled: "No longer planned"
        }
    }
}

struct PreviewIssue: Identifiable {
    let number: Int
    let title: String
    let status: IssueStatus
    let priority: Int
    let project: String
    let label: String
    let type: String
    var id: String { "DEMO-\(number)" }
    var labelColor: Color { label == "design" ? .purple : Design.accent }
    var description: String {
        "This is a fictional issue in the native InfTask preview. It is not connected to your workspace.\n\nExplore the same compact layout, familiar status icons and quiet surfaces — now drawn entirely with SwiftUI."
    }

    static let samples: [PreviewIssue] = [
        .init(number: 12, title: "Explore a native command menu", status: .triage, priority: 0, project: "Mac experience", label: "design", type: "feature"),
        .init(number: 11, title: "Keep workspace context across windows", status: .backlog, priority: 4, project: "Mac experience", label: "desktop", type: "improvement"),
        .init(number: 10, title: "Refine keyboard navigation", status: .todo, priority: 3, project: "Mac experience", label: "desktop", type: "improvement"),
        .init(number: 9, title: "Make empty states useful", status: .todo, priority: 4, project: "Delivery board", label: "design", type: "feature"),
        .init(number: 8, title: "Bring the board to macOS", status: .agentReady, priority: 2, project: "Mac experience", label: "desktop", type: "feature"),
        .init(number: 7, title: "Polish status and priority icons", status: .agentReady, priority: 3, project: "Delivery board", label: "design", type: "improvement"),
        .init(number: 6, title: "Build the native application shell", status: .inProgress, priority: 2, project: "Mac experience", label: "desktop", type: "feature"),
        .init(number: 5, title: "Match light and dark surfaces", status: .inProgress, priority: 3, project: "Delivery board", label: "design", type: "improvement"),
        .init(number: 4, title: "Review the issue detail layout", status: .inReview, priority: 3, project: "Delivery board", label: "design", type: "feature"),
        .init(number: 3, title: "Define shared spacing and typography", status: .done, priority: 0, project: "Delivery board", label: "design", type: "chore"),
        .init(number: 2, title: "Sketch the workspace sidebar", status: .done, priority: 0, project: "Mac experience", label: "desktop", type: "feature"),
        .init(number: 1, title: "Embed the website in a desktop window", status: .canceled, priority: 0, project: "Mac experience", label: "desktop", type: "feature")
    ]
}

enum Destination: Hashable {
    case all, status(IssueStatus), project(String), labels
    var title: String {
        switch self {
        case .all: "All issues"
        case .status(let status): status.rawValue
        case .project(let name): name
        case .labels: "Labels"
        }
    }
    func matches(_ issue: PreviewIssue) -> Bool {
        switch self {
        case .all, .labels: true
        case .status(let status): issue.status == status
        case .project(let name): issue.project == name
        }
    }
}
