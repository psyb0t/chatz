package fixedresponses

import "time"

const (
	// ShowcasePromptOperations is a recording-ready request for an operational
	// MCP that exposes services, events, and logs.
	ShowcasePromptOperations = "Show me what's happening across the " +
		"production platform right now."
	// ShowcasePromptSales is a recording-ready request for a sales MCP.
	ShowcasePromptSales = "Where are we losing deals in the sales pipeline?"
	// ShowcasePromptCustomers is a recording-ready request for a customer-data
	// MCP that exposes accounts, activity, and support events.
	ShowcasePromptCustomers = "Which customers are at risk and who should " +
		"the team contact first?"

	fileShowcaseOperations = "showcase-operations.txt"
	fileShowcaseSales      = "showcase-sales.txt"
	fileShowcaseCustomers  = "showcase-customers.txt"

	showcaseUnavailableText = "The requested showcase response is unavailable."

	showcaseInitialDelay   = 700 * time.Millisecond
	showcaseStepDelay      = 450 * time.Millisecond
	showcaseToolFirstDelay = 1200 * time.Millisecond
	showcaseToolNextDelay  = 1500 * time.Millisecond
	// 6 runes per delta at 18ms ≈ 330 chars/s (~80 tok/s): a smooth,
	// word-by-word stream that reads like a fast premium model without
	// letting the dashboard spec block drag.
	showcaseTextChunkDelay = 18 * time.Millisecond

	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseOperationsThinking = "I’ll check service health and recent platform events, then summarize the current operating picture."
	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseSalesThinking = "I’ll inspect stage conversion and recent loss reasons before mapping where the pipeline is leaking."
	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseCustomersThinking = "I’ll combine account health signals with recent support activity to prioritize the outreach list."

	//nolint:lll // exact recording prompt matched verbatim
	ShowcasePromptInfrastructure = "How is the platform infrastructure holding up under load right now?"
	//nolint:lll // exact recording prompt matched verbatim
	ShowcasePromptExperiment = "How did the new onboarding experiment perform against control?"
	//nolint:lll // exact recording prompt matched verbatim
	ShowcasePromptFinance = "Where is our cloud spend going this quarter and are we on budget?"
	//nolint:lll // exact recording prompt matched verbatim
	ShowcasePromptReliability = "Walk me through the active incident and its blast radius."

	fileShowcaseInfrastructure = "showcase-infrastructure.txt"
	fileShowcaseExperiment     = "showcase-experiment.txt"
	fileShowcaseFinance        = "showcase-finance.txt"
	fileShowcaseReliability    = "showcase-reliability.txt"

	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseInfrastructureThinking = "I’ll pull current cluster saturation and the per-region latency matrix, then flag where we’re closest to capacity."
	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseExperimentThinking = "I’ll compare the variant against control on conversion and activation time, then check whether longer sessions actually convert better."
	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseFinanceThinking = "I’ll break spend down by category and by service, then check the current burn against the quarterly budget."
	//nolint:lll // Recording copy stays as one sentence so streamed thinking reads naturally.
	showcaseReliabilityThinking = "I’ll pull the active incident’s affected services and the dependency graph to see the blast radius, then walk the event timeline."
)

type showcaseResponse struct {
	filename string
	steps    []Step
	evidence []showcaseEvidence
}

type showcaseEvidence struct {
	toolResult string
	answer     string
}

// showcaseResponses maps exact, recording-ready user messages to the embedded
// result that an MCP-backed assistant could return. The catalog is deliberately
// closed: user input never selects a file, template, tool, or upstream request.
//
//nolint:gochecknoglobals // build-time catalog
var showcaseResponses = map[string]showcaseResponse{
	ShowcasePromptOperations: {
		filename: fileShowcaseOperations,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseOperationsThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_operations_health",
					Name:        "operations__get_service_health",
					ArgsJSON:    `{"scope":"production"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "window": "last_5_minutes",
  "request_rate_per_min": 18400,
  "request_rate_delta_percent": 6,
  "error_rate_percent": 0.7,
  "error_rate_delta_percentage_points": -1,
  "p95_latency_ms": 284,
  "p95_latency_delta_ms": 12,
  "services": [
    {
      "name": "gateway",
      "status": "healthy",
      "signal": "steady throughput",
      "next_action": "observe"
    },
    {
      "name": "api",
      "status": "healthy",
      "signal": "p95 within target",
      "next_action": "observe"
    },
    {
      "name": "worker",
      "status": "watch",
      "signal": "retry queue elevated",
      "retry_queue": 14,
      "retry_queue_alert_threshold": 20,
      "next_action": "inspect delayed jobs"
    },
    {
      "name": "indexer",
      "status": "healthy",
      "signal": "caught up",
      "next_action": "observe"
    }
  ],
  "request_volume_k_per_min": {
    "gateway": [9.1, 9.8, 9.5, 10.2, 10.6],
    "workers": [5.2, 5.8, 5.5, 6.9, 7.8]
  }
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_operations_events",
					Name:        "operations__list_recent_events",
					ArgsJSON:    `{"scope":"production","limit":8}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "retries_by_service": {
    "gateway": 3,
    "api": 7,
    "worker": 14,
    "indexer": 4
  },
  "events": [
    {
      "time": "T-00:03",
      "level": "info",
      "source": "gateway",
      "message": "traffic window opened"
    },
    {
      "time": "T-00:02",
      "level": "warn",
      "source": "worker",
      "message": "retry queue above normal band"
    },
    {
      "time": "T-00:01",
      "level": "info",
      "source": "scheduler",
      "message": "rebalanced delayed jobs"
    },
    {
      "time": "Now",
      "level": "info",
      "source": "api",
      "message": "health checks remain green"
    }
  ],
  "recommended_action": "inspect delayed-job cohort"
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"request_rate_per_min": 18400`,
				answer:     `"value":"18.4k"`,
			},
			{
				toolResult: `"request_rate_delta_percent": 6`,
				answer:     `"delta":6`,
			},
			{
				toolResult: `"error_rate_percent": 0.7`,
				answer:     `"value":"0.7"`,
			},
			{
				toolResult: `"error_rate_delta_percentage_points": -1`,
				answer:     `"delta":-1`,
			},
			{
				toolResult: `"p95_latency_ms": 284`,
				answer:     `"value":"284"`,
			},
			{
				toolResult: `"p95_latency_delta_ms": 12`,
				answer:     `"delta":12`,
			},
			{
				toolResult: `"next_action": "inspect delayed jobs"`,
				answer:     `"inspect delayed jobs"`,
			},
			{
				toolResult: `"worker": 14`,
				answer:     `"values":[3,7,14,4]`,
			},
			{
				toolResult: `"retry_queue_alert_threshold": 20`,
				answer:     `"max":20`,
			},
			{
				toolResult: `"message": "traffic window opened"`,
				answer:     `"message":"traffic window opened"`,
			},
			{
				toolResult: `"recommended_action":`,
				answer:     "inspect the delayed-job cohort",
			},
		},
	},
	ShowcasePromptSales: {
		filename: fileShowcaseSales,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseSalesThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_sales_conversion",
					Name:        "sales__get_stage_conversion",
					ArgsJSON:    `{"period":"current_quarter"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "period": "current_quarter",
  "open_pipeline_usd": 1280000,
  "pipeline_trend_millions": [0.94, 1.02, 1.08, 1.17, 1.22, 1.28],
  "qualified_to_won_percent": 19,
  "qualified_to_won_delta_percentage_points": 0,
  "stages": [
    {"label": "Qualified", "value": 128},
    {"label": "Discovery complete", "value": 92},
    {"label": "Solution evaluated", "value": 57},
    {"label": "Proposal sent", "value": 39},
    {"label": "Won", "value": 24}
  ],
  "source_conversion": {
    "Partner": {"qualified": 38, "won": 9},
    "Inbound": {"qualified": 34, "won": 7},
    "Outbound": {"qualified": 27, "won": 3},
    "Expansion": {"qualified": 29, "won": 5}
  }
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID: "toolu_showcase_sales_losses",
					Name:      "sales__summarize_closed_lost",
					ArgsJSON: `{
"period": "current_quarter",
"group_by": "reason"
}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "deals_analyzed": 46,
  "top_loss_reason": "no decision",
  "stalled_deals": 14,
  "stalled_deals_delta": -4,
  "opportunities": [
    {
      "name": "Northstar renewal",
      "stage": "solution evaluated",
      "signal": "no activity in 8 days",
      "move": "book technical review"
    },
    {
      "name": "Harbor expansion",
      "stage": "discovery complete",
      "signal": "champion engaged",
      "move": "send mutual plan"
    },
    {
      "name": "Summit rollout",
      "stage": "proposal sent",
      "signal": "legal review open",
      "move": "confirm decision date"
    },
    {
      "name": "Cedar pilot",
      "stage": "solution evaluated",
      "signal": "usage questions rising",
      "move": "bring solutions engineer"
    }
  ]
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"pipeline_trend_millions": [`,
				answer:     `"values":[0.94,1.02,1.08,1.17,1.22,1.28]`,
			},
			{
				toolResult: `"qualified_to_won_percent": 19`,
				answer:     `"value":"19"`,
			},
			{
				toolResult: `"qualified_to_won_delta_percentage_points": 0`,
				answer:     `"delta":0`,
			},
			{
				toolResult: `"stalled_deals": 14`,
				answer:     `"value":"14"`,
			},
			{
				toolResult: `"stalled_deals_delta": -4`,
				answer:     `"delta":-4`,
			},
			{
				toolResult: `"label": "Qualified", "value": 128`,
				answer:     `{"label":"Qualified","value":128}`,
			},
			{
				toolResult: `"name": "Northstar renewal"`,
				answer:     `"Northstar renewal"`,
			},
			{
				toolResult: `"top_loss_reason": "no decision"`,
				answer:     "no decision as the leading loss reason",
			},
		},
	},
	ShowcasePromptCustomers: {
		filename: fileShowcaseCustomers,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseCustomersThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_customers_health",
					Name:        "customers__list_at_risk_accounts",
					ArgsJSON:    `{"limit":10,"sort":"risk_desc"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "at_risk_accounts": 7,
  "at_risk_accounts_delta": -2,
  "portfolio_adoption_percent": 74,
  "engagement": {
    "healthy_cohort": [82, 85, 87, 89],
    "at_risk_cohort": [24, 21, 18, 15]
  },
  "risk_signal_mix": {
    "falling_activity": 4,
    "support_friction": 2,
    "renewal_timing": 1
  },
  "priority_accounts": [
    {
      "name": "Atlas team",
      "signal": "activity down; open issue",
      "owner": "success lead",
      "outreach": "schedule adoption review"
    },
    {
      "name": "Juniper group",
      "signal": "renewal approaching",
      "owner": "account lead",
      "outreach": "confirm outcomes and timeline"
    },
    {
      "name": "River workspace",
      "signal": "support friction",
      "owner": "support lead",
      "outreach": "send resolution recap"
    }
  ]
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_customers_support",
					Name:        "support__summarize_open_cases",
					ArgsJSON:    `{"account_scope":"at_risk","status":"open"}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "open_support_threads": 17,
  "thread_trend": [31, 29, 27, 25, 23, 17],
  "timeline": [
    {
      "time": "T-5d",
      "label": "Activity decline detected",
      "detail": "Atlas team fell below its usual usage band"
    },
    {
      "time": "T-2d",
      "label": "Support thread reopened",
      "detail": "River workspace requested implementation help"
    },
    {
      "time": "Today",
      "label": "Outreach queue prepared",
      "detail": "Three owners have a clear next action"
    }
  ],
  "recommended_order": ["Atlas team", "Juniper group", "River workspace"]
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"at_risk_accounts": 7`,
				answer:     `"value":"7"`,
			},
			{
				toolResult: `"at_risk_accounts_delta": -2`,
				answer:     `"delta":-2`,
			},
			{
				toolResult: `"portfolio_adoption_percent": 74`,
				answer:     `"value":74`,
			},
			{
				toolResult: `"healthy_cohort": [82, 85, 87, 89]`,
				answer:     `"y":82`,
			},
			{
				toolResult: `"at_risk_cohort": [24, 21, 18, 15]`,
				answer:     `"y":15`,
			},
			{
				toolResult: `"falling_activity": 4`,
				answer:     `"value":4`,
			},
			{
				toolResult: `"name": "Atlas team"`,
				answer:     "Atlas team",
			},
			{
				toolResult: `"recommended_order": [`,
				answer:     "followed by River",
			},
			{
				toolResult: `"open_support_threads": 17`,
				answer:     `"value":"17"`,
			},
		},
	},
	ShowcasePromptInfrastructure: {
		filename: fileShowcaseInfrastructure,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseInfrastructureThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_infra_saturation",
					Name:        "infra__get_cluster_saturation",
					ArgsJSON:    `{"scope":"production"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "window": "last_5_minutes",
  "cpu_saturation_percent": 78,
  "memory_saturation_percent": 71,
  "throughput_rps": 41200,
  "throughput_rps_delta_percent": 8,
  "throughput_krps_trend": {
    "gateway": [34.1, 36.4, 35.2, 39.8, 41.2],
    "workers": [12.3, 13.1, 12.8, 14.6, 15.9]
  },
  "top_services": [
    {
      "name": "checkout-api",
      "cpu_percent": 86,
      "signal": "hot path, autoscaling",
      "action": "add 2 replicas"
    },
    {
      "name": "search-indexer",
      "cpu_percent": 74,
      "signal": "batch backlog",
      "action": "throttle reindex"
    },
    {
      "name": "media-transcoder",
      "cpu_percent": 69,
      "signal": "queue draining",
      "action": "observe"
    }
  ]
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_infra_latency",
					Name:        "infra__get_regional_latency",
					ArgsJSON:    `{"window":"last_hour"}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "unit": "ms_p95",
  "intervals": ["T-5", "T-4", "T-3", "T-2", "Now"],
  "regions": {
    "us-east": [180, 176, 182, 190, 188],
    "us-west": [196, 205, 210, 231, 244],
    "eu-west": [210, 214, 208, 219, 222],
    "ap-south": [268, 279, 291, 305, 318]
  },
  "worst_region": "ap-south",
  "recommended_action": "shift ap-south reads to the eu-west replica"
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"cpu_saturation_percent": 78`,
				answer:     `"value":78`,
			},
			{
				toolResult: `"memory_saturation_percent": 71`,
				answer:     `"value":71`,
			},
			{
				toolResult: `"throughput_rps_delta_percent": 8`,
				answer:     `"delta":8`,
			},
			{
				toolResult: `"name": "checkout-api"`,
				answer:     "checkout-api",
			},
			{
				toolResult: `"worst_region": "ap-south"`,
				answer:     "ap-south",
			},
			{
				toolResult: `"recommended_action":`,
				answer:     "shift ap-south read traffic",
			},
		},
	},
	ShowcasePromptExperiment: {
		filename: fileShowcaseExperiment,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseExperimentThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_exp_variants",
					Name:        "experiments__get_variant_results",
					ArgsJSON:    `{"experiment":"onboarding_v2"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "experiment": "onboarding_v2",
  "status": "significant",
  "control_conversion_percent": 22.4,
  "variant_conversion_percent": 27.1,
  "lift_percent": 21,
  "p_value": 0.008,
  "sample_size": 48200,
  "activation_minutes_by_variant": {
    "control": {"min": 2, "q1": 6, "median": 11, "q3": 19, "max": 34},
    "variant": {"min": 1, "q1": 4, "median": 7, "q3": 12, "max": 26}
  }
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_exp_sessions",
					Name:        "analytics__get_session_distribution",
					ArgsJSON:    `{"experiment":"onboarding_v2"}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "session_minutes_histogram": [
    {"label": "0-2", "value": 640},
    {"label": "2-5", "value": 1820},
    {"label": "5-10", "value": 2470},
    {"label": "10-20", "value": 1310},
    {"label": "20-40", "value": 420},
    {"label": "40+", "value": 130}
  ],
  "session_vs_conversion": [
    {"minutes": 3, "conversion_percent": 12},
    {"minutes": 9, "conversion_percent": 33},
    {"minutes": 14, "conversion_percent": 41},
    {"minutes": 35, "conversion_percent": 29}
  ],
  "insight": "conversion peaks around 12-15 minutes, then declines"
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"variant_conversion_percent": 27.1`,
				answer:     `"value":"27.1"`,
			},
			{
				toolResult: `"lift_percent": 21`,
				answer:     `"delta":21`,
			},
			{
				toolResult: `"p_value": 0.008`,
				answer:     "0.008",
			},
			{
				toolResult: `"median": 7`,
				answer:     `"median":7`,
			},
			{
				toolResult: `"label": "5-10", "value": 2470`,
				answer:     `"label":"5-10","value":2470`,
			},
			{
				toolResult: `"insight":`,
				answer:     "peaks around",
			},
		},
	},
	ShowcasePromptFinance: {
		filename: fileShowcaseFinance,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseFinanceThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_fin_spend",
					Name:        "finance__get_spend_breakdown",
					ArgsJSON:    `{"period":"current_quarter"}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "period": "current_quarter",
  "total_spend_usd": 412000,
  "by_category": [
    {"category": "Compute", "usd": 186000},
    {"category": "Storage", "usd": 92000},
    {"category": "Data transfer", "usd": 58000},
    {"category": "Managed DB", "usd": 44000},
    {"category": "Observability", "usd": 32000}
  ],
  "by_service": [
    {"service": "checkout-api", "usd": 74000, "group": "Compute"},
    {"service": "media-store", "usd": 68000, "group": "Storage"},
    {"service": "search-cluster", "usd": 61000, "group": "Compute"},
    {"service": "cdn-egress", "usd": 41000, "group": "Data transfer"}
  ],
  "monthly_trend_usd_k": [122, 131, 159]
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_fin_budget",
					Name:        "finance__get_budget_status",
					ArgsJSON:    `{"period":"current_quarter"}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "period": "current_quarter",
  "budget_usd": 450000,
  "spent_usd": 412000,
  "forecast_usd": 486000,
  "variance_percent": 8,
  "status": "over_forecast",
  "top_overrun": "media-store"
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"total_spend_usd": 412000`,
				answer:     "$412k",
			},
			{
				toolResult: `"variance_percent": 8`,
				answer:     `"delta":8`,
			},
			{
				toolResult: `"category": "Compute", "usd": 186000`,
				answer:     `"label":"Compute","value":186`,
			},
			{
				toolResult: `"forecast_usd": 486000`,
				answer:     "$486k",
			},
			{
				toolResult: `"top_overrun": "media-store"`,
				answer:     "media-store",
			},
		},
	},
	ShowcasePromptReliability: {
		filename: fileShowcaseReliability,
		steps: []Step{
			{Kind: StepThinking, Text: showcaseReliabilityThinking},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_inc_active",
					Name:        "incidents__get_active",
					ArgsJSON:    `{}`,
					ResultDelay: showcaseToolFirstDelay,
					ResultText: `{
  "incident_id": "INC-2043",
  "severity": "SEV2",
  "opened": "T-00:22",
  "summary": "elevated 5xx on checkout from a payments slowdown",
  "affected_services": [
    {
      "service": "payments-gateway",
      "status": "degraded",
      "error_rate_percent": 6.1
    },
    {
      "service": "checkout-api",
      "status": "degraded",
      "error_rate_percent": 4.8
    },
    {
      "service": "cart-service",
      "status": "watch",
      "error_rate_percent": 1.2
    },
    {
      "service": "order-worker",
      "status": "watch",
      "error_rate_percent": 0.9
    }
  ],
  "error_budget_burn_percent": 34,
  "events": [
    {
      "time": "T-00:22",
      "level": "error",
      "source": "payments-gateway",
      "message": "upstream timeout rate crossed 5%"
    },
    {
      "time": "T-00:09",
      "level": "info",
      "source": "oncall",
      "message": "failover to secondary payments region initiated"
    }
  ]
}`,
				},
			},
			{
				Kind:        StepTool,
				DelayBefore: showcaseStepDelay,
				Tool: &ToolStep{
					ToolUseID:   "toolu_showcase_inc_topology",
					Name:        "topology__get_service_graph",
					ArgsJSON:    `{"scope":"affected"}`,
					ResultDelay: showcaseToolNextDelay,
					ResultText: `{
  "nodes": [
    {"id": "gw", "label": "gateway", "group": "edge"},
    {"id": "checkout", "label": "checkout-api", "group": "core"},
    {"id": "pay", "label": "payments-gateway", "group": "external"},
    {"id": "cart", "label": "cart-service", "group": "core"},
    {"id": "order", "label": "order-worker", "group": "core"},
    {"id": "db", "label": "orders-db", "group": "data"}
  ],
  "edges": [
    {"source": "gw", "target": "checkout", "calls_per_s": 1800},
    {"source": "checkout", "target": "pay", "calls_per_s": 900},
    {"source": "checkout", "target": "cart", "calls_per_s": 1600}
  ],
  "root_cause_node": "pay"
}`,
				},
			},
		},
		evidence: []showcaseEvidence{
			{
				toolResult: `"severity": "SEV2"`,
				answer:     `"value":"SEV2"`,
			},
			{
				toolResult: `"error_budget_burn_percent": 34`,
				answer:     `"value":"34"`,
			},
			{
				toolResult: `"service": "payments-gateway"`,
				answer:     "payments-gateway",
			},
			{
				toolResult: `"root_cause_node": "pay"`,
				answer:     `"id":"pay"`,
			},
			{
				toolResult: `"failover to secondary payments region initiated"`,
				answer:     "failover to secondary payments region initiated",
			},
		},
	},
}

// GetShowcase resolves a deterministic, durable showcase reply for an exact
// catalog match. All other text remains a normal chat turn.
func GetShowcase(text string) (Response, bool) {
	showcase, ok := showcaseResponses[text]
	if !ok {
		return Response{}, false
	}

	data, err := fs.ReadFile(showcase.filename)
	if err != nil {
		return Response{
			Kind:    KindText,
			Text:    showcaseUnavailableText,
			Persist: true,
		}, true
	}

	steps := append([]Step(nil), showcase.steps...)
	steps = append(steps, Step{
		Kind:        StepText,
		DelayBefore: showcaseStepDelay,
		Text:        string(data),
	})

	return Response{
		Kind:           KindTools,
		Steps:          steps,
		Persist:        true,
		InitialDelay:   showcaseInitialDelay,
		TextChunkDelay: showcaseTextChunkDelay,
	}, true
}
