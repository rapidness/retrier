package rule

// MatchLogic evaluates AND/OR/NOT logic combinations.
func MatchLogic(logic *LogicMatch, ctx *ResponseContext) (bool, error) {
	if logic == nil {
		return true, nil // no logic → always matches
	}

	// AND: all must match
	if len(logic.And) > 0 {
		for _, sub := range logic.And {
			ok, err := evaluateMatchSpec(&sub, ctx)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	}

	// OR: at least one must match
	if len(logic.Or) > 0 {
		for _, sub := range logic.Or {
			ok, err := evaluateMatchSpec(&sub, ctx)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	}

	// NOT: negate the inner match
	if logic.Not != nil {
		ok, err := evaluateMatchSpec(logic.Not, ctx)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}

	return true, nil
}
