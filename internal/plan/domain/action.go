package domain

import "fmt"

// ActionType is the normalized operation OpenTofu will perform on a resource.
type ActionType string

const (
	ActionCreate  ActionType = "create"
	ActionUpdate  ActionType = "update"
	ActionDelete  ActionType = "delete"
	ActionReplace ActionType = "replace"
	ActionNoOp    ActionType = "no-op"
)

// NormalizeAction translates an OpenTofu action sequence into the domain action.
// Replacement is accepted only for the two ordered pairs emitted by OpenTofu.
func NormalizeAction(actions []string) (ActionType, error) {
	switch len(actions) {
	case 0:
		return ActionNoOp, nil
	case 1:
		switch actions[0] {
		case "create":
			return ActionCreate, nil
		case "update":
			return ActionUpdate, nil
		case "delete":
			return ActionDelete, nil
		case "no-op", "read":
			return ActionNoOp, nil
		default:
			return "", fmt.Errorf("unsupported action %q", actions[0])
		}
	case 2:
		if (actions[0] == "delete" && actions[1] == "create") ||
			(actions[0] == "create" && actions[1] == "delete") {
			return ActionReplace, nil
		}
		return "", fmt.Errorf("unsupported action sequence %q", actions)
	default:
		return "", fmt.Errorf("unsupported action sequence %q", actions)
	}
}
