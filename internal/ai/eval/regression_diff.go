package eval

type caseOutcome CaseSnapshot

func buildRegressionReport(baselineRunID, currentRunID string, baseline, current map[string]caseOutcome) *RegressionReport {
	report := &RegressionReport{
		BaselineRunID: baselineRunID,
		CurrentRunID:  currentRunID,
	}
	if baseline == nil {
		baseline = map[string]caseOutcome{}
	}
	if current == nil {
		current = map[string]caseOutcome{}
	}

	for caseID, cur := range current {
		base, exists := baseline[caseID]
		if !exists {
			if !cur.Match {
				report.NewFailures = append(report.NewFailures, RegressionChange{
					CaseID:   caseID,
					Question: cur.Question,
					WasMatch: true,
					IsMatch:  false,
					IsReason: cur.Reason,
				})
			}
			continue
		}
		appendRegressionChange(report, caseID, base, cur)
	}

	for caseID, base := range baseline {
		if _, exists := current[caseID]; !exists && !base.Match {
			report.FixedFailures = append(report.FixedFailures, RegressionChange{
				CaseID:    caseID,
				Question:  base.Question,
				WasMatch:  false,
				IsMatch:   true,
				WasReason: base.Reason,
			})
		}
	}
	return report
}

func appendRegressionChange(report *RegressionReport, caseID string, base, cur caseOutcome) {
	switch {
	case base.Match && !cur.Match:
		report.NewFailures = append(report.NewFailures, RegressionChange{
			CaseID:    caseID,
			Question:  cur.Question,
			WasMatch:  true,
			IsMatch:   false,
			WasReason: base.Reason,
			IsReason:  cur.Reason,
		})
	case !base.Match && cur.Match:
		report.FixedFailures = append(report.FixedFailures, RegressionChange{
			CaseID:    caseID,
			Question:  cur.Question,
			WasMatch:  false,
			IsMatch:   true,
			WasReason: base.Reason,
		})
	case base.Reason != cur.Reason:
		report.ChangedCases = append(report.ChangedCases, RegressionChange{
			CaseID:    caseID,
			Question:  cur.Question,
			WasMatch:  base.Match,
			IsMatch:   cur.Match,
			WasReason: base.Reason,
			IsReason:  cur.Reason,
		})
	}
}
