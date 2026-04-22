function export_viz_model(wiring::Dict{String,Any}, petri::Dict{String,Any}, event::Dict{String,Any}, version::Int)
    repo = asdict(getd(wiring, "repo", Dict()))
    nodes = deepcopy(vecdict(getd(wiring, "nodes", [])))
    edges = deepcopy(vecdict(getd(wiring, "edges", [])))

    # Add Petri places/transitions as a separate visual layer. The frontend can filter by mode.
    for p in vecdict(getd(petri, "places", []))
        push!(nodes, Dict{String,Any}(
            "id" => getd(p, "id", ""),
            "label" => getd(p, "label", ""),
            "kind" => "PetriPlace",
            "status" => token_on_place(petri, getd(p, "id", "")) ? "active" : "stable",
            "metadata" => p,
        ))
    end
    for t in vecdict(getd(petri, "transitions", []))
        push!(nodes, Dict{String,Any}(
            "id" => getd(t, "id", ""),
            "label" => getd(t, "label", ""),
            "kind" => "PetriTransition",
            "status" => transition_status(event, getd(t, "id", "")),
            "metadata" => t,
        ))
    end
    i = 0
    for a in vecdict(getd(petri, "arcs", []))
        i += 1
        push!(edges, Dict{String,Any}(
            "id" => "petri-arc:" * string(i) * ":" * string(getd(a,"source","")) * "->" * string(getd(a,"target","")),
            "source" => getd(a, "source", ""),
            "target" => getd(a, "target", ""),
            "kind" => "petri-arc",
            "status" => "stable",
            "metadata" => a,
        ))
    end
    for token in vecdict(getd(petri, "tokens", []))
        tid = getd(token, "id", safe_id("token", uuid4()))
        place = getd(token, "place", "")
        push!(nodes, Dict{String,Any}(
            "id" => tid,
            "label" => getd(token, "label", "event"),
            "kind" => "PetriToken",
            "status" => "active",
            "metadata" => token,
        ))
        push!(edges, Dict{String,Any}(
            "id" => "token-at:" * string(tid) * "->" * string(place),
            "source" => tid,
            "target" => place,
            "kind" => "token-at",
            "status" => "active",
        ))
    end

    if !isempty(event)
        augment_with_event!(nodes, edges, event, repo)
    end

    return Dict{String,Any}(
        "kind" => "HybridRepositoryDiagram",
        "repo" => repo,
        "model_version" => version,
        "nodes" => nodes,
        "edges" => edges,
        "layout_hints" => Dict(
            "rank_direction" => "LR",
            "modes" => ["topology", "petri", "hybrid"],
            "changed_statuses" => ["added", "modified", "removed", "changed", "active", "failed"]
        )
    )
end

function token_on_place(petri::Dict{String,Any}, place_id)
    for t in vecdict(getd(petri, "tokens", []))
        if getd(t, "place", "") == place_id
            return true
        end
    end
    return false
end

function transition_status(event::Dict{String,Any}, transition_id)
    isempty(event) && return "stable"
    evname = String(getd(event, "event", ""))
    conclusion = String(getd(event, "conclusion", ""))
    if evname == "push" && transition_id == "transition:Push"
        return "active"
    elseif evname == "pull_request" && transition_id in ["transition:OpenPR", "transition:MergePR"]
        return "active"
    elseif evname in ["check_run", "check_suite", "workflow_run"]
        if conclusion == "success" && transition_id == "transition:PassCI"
            return "active"
        elseif conclusion != "" && transition_id == "transition:FailCI"
            return "failed"
        elseif transition_id == "transition:StartCI"
            return "active"
        end
    elseif evname == "release" && transition_id == "transition:CreateRelease"
        return "active"
    end
    return "stable"
end

function augment_with_event!(nodes, edges, event::Dict{String,Any}, repo::Dict{String,Any})
    delivery = String(getd(event, "delivery_id", string(uuid4())))
    evname = String(getd(event, "event", "event"))
    action = String(getd(event, "action", ""))
    repo_id = int_or(0, getd(repo, "id", dig(event, ["repository", "id"], 0)))
    event_id = safe_id("event", delivery)
    label = isempty(action) ? evname : evname * ":" * action
    push!(nodes, Dict{String,Any}(
        "id" => event_id,
        "label" => label,
        "kind" => "GitHubEvent",
        "status" => event_status(event),
        "metadata" => event,
    ))
    push!(edges, Dict{String,Any}(
        "id" => "event-target:" * event_id,
        "source" => event_id,
        "target" => safe_id("repo", repo_id),
        "kind" => "observed-event",
        "status" => event_status(event),
    ))

    if evname == "pull_request"
        prn = int_or(0, getd(event, "pull_request_number", 0))
        if prn > 0
            pr_id = safe_id("pr", prn)
            push!(nodes, Dict("id"=>pr_id, "label"=>"PR #"*string(prn), "kind"=>"PullRequest", "status"=>event_status(event), "metadata"=>event))
            push!(edges, Dict("id"=>"pr-event:"*pr_id*"->"*event_id, "source"=>pr_id, "target"=>event_id, "kind"=>"pr-event", "status"=>event_status(event)))
        end
    elseif evname in ["check_run", "check_suite", "workflow_run"]
        name = getd(event, "workflow_name", getd(event, "check_name", "CI"))
        ci_id = safe_id("ci", string(name) * ":" * string(getd(event, "head_sha", "")))
        push!(nodes, Dict("id"=>ci_id, "label"=>name, "kind"=>"CIStatus", "status"=>event_status(event), "metadata"=>event))
        push!(edges, Dict("id"=>"ci-event:"*ci_id*"->"*event_id, "source"=>ci_id, "target"=>event_id, "kind"=>"ci-event", "status"=>event_status(event)))
    end
end

function event_status(event::Dict{String,Any})
    conclusion = String(getd(event, "conclusion", ""))
    status = String(getd(event, "status", ""))
    if conclusion == "success"
        return "passed"
    elseif conclusion != ""
        return "failed"
    elseif status in ["queued", "in_progress", "requested"]
        return "active"
    else
        return "changed"
    end
end
