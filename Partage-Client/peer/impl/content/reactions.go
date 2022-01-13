package content

type Reaction int

const (
	HAPPY Reaction = iota
	ANGRY
	CONFUSED
	APPROVE
	DISAPPROVE
)

func (r Reaction) String() string {
	switch r {
	case HAPPY:
		return "happy"
	case ANGRY:
		return "angry"
	case CONFUSED:
		return "confused"
	case APPROVE:
		return "approve"
	case DISAPPROVE:
		return "disapprove"
	}
	return "unknown"
}


