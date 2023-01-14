package permission

func ParsePermissionClaim(v any, ok bool) []string {
	if !ok {
		return nil
	}
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	ss := make([]string, 0, len(xs))
	for _, x := range xs {
		if s, ok := x.(string); ok {
			ss = append(ss, s)
		}
	}
	return ss
}
