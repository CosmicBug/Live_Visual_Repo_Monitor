function build_petri_model(event::Dict{String,Any}, snapshot::Dict{String,Any})
    places = [
        Dict("id"=>"place:CleanMain", "label"=>"Clean main"),
        Dict("id"=>"place:BranchChanged", "label"=>"Branch changed"),
        Dict("id"=>"place:PROpen", "label"=>"PR open"),
        Dict("id"=>"place:ReviewPending", "label"=>"Review pending"),
        Dict("id"=>"place:CIRunning", "label"=>"CI running"),
        Dict("id"=>"place:CIPassed", "label"=>"CI passed"),
        Dict("id"=>"place:CIFailed", "label"=>"CI failed"),
        Dict("id"=>"place:MergeReady", "label"=>"Merge ready"),
        Dict("id"=>"place:Merged", "label"=>"Merged"),
        Dict("id"=>"place:Released", "label"=>"Released"),
        Dict("id"=>"place:SecurityAlertOpen", "label"=>"Security alert"),
    ]
    transitions = [
        Dict("id"=>"transition:Push", "label"=>"Push"),
        Dict("id"=>"transition:OpenPR", "label"=>"Open PR"),
        Dict("id"=>"transition:Review", "label"=>"Review"),
        Dict("id"=>"transition:StartCI", "label"=>"Start CI"),
        Dict("id"=>"transition:PassCI", "label"=>"Pass CI"),
        Dict("id"=>"transition:FailCI", "label"=>"Fail CI"),
        Dict("id"=>"transition:MergePR", "label"=>"Merge PR"),
        Dict("id"=>"transition:CreateRelease", "label"=>"Release"),
        Dict("id"=>"transition:OpenSecurityAlert", "label"=>"Security alert"),
    ]
    arcs = [
        Dict("source"=>"place:CleanMain", "target"=>"transition:Push"),
        Dict("source"=>"transition:Push", "target"=>"place:BranchChanged"),
        Dict("source"=>"place:BranchChanged", "target"=>"transition:OpenPR"),
        Dict("source"=>"transition:OpenPR", "target"=>"place:PROpen"),
        Dict("source"=>"place:PROpen", "target"=>"transition:StartCI"),
        Dict("source"=>"transition:StartCI", "target"=>"place:CIRunning"),
        Dict("source"=>"place:CIRunning", "target"=>"transition:PassCI"),
        Dict("source"=>"transition:PassCI", "target"=>"place:CIPassed"),
        Dict("source"=>"place:CIRunning", "target"=>"transition:FailCI"),
        Dict("source"=>"transition:FailCI", "target"=>"place:CIFailed"),
        Dict("source"=>"place:CIPassed", "target"=>"transition:MergePR"),
        Dict("source"=>"transition:MergePR", "target"=>"place:Merged"),
        Dict("source"=>"place:Merged", "target"=>"transition:CreateRelease"),
        Dict("source"=>"transition:CreateRelease", "target"=>"place:Released"),
    ]

    tokens = Vector{Dict{String,Any}}()
    if !isempty(event)
        evname = String(getd(event, "event", "event"))
        delivery = String(getd(event, "delivery_id", string(uuid4())))
        target_place = event_place(event)
        push!(tokens, Dict{String,Any}(
            "id" => safe_id("token", delivery),
            "label" => evname,
            "place" => target_place,
            "metadata" => event,
        ))
    end

    return Dict{String,Any}(
        "kind" => "AlgebraicPetriCompatibleNet",
        "places" => places,
        "transitions" => transitions,
        "arcs" => arcs,
        "tokens" => tokens,
        "metadata" => Dict(
            "ecosystem" => "AlgebraicJulia",
            "package" => "AlgebraicPetri.jl",
            "interpretation" => "GitHub events are transitions; active repository states are token places."
        )
    )
end

function event_place(event::Dict{String,Any})
    evname = String(getd(event, "event", ""))
    action = String(getd(event, "action", ""))
    status = String(getd(event, "status", ""))
    conclusion = String(getd(event, "conclusion", ""))
    if evname == "push"
        return "place:BranchChanged"
    elseif evname == "pull_request" && action == "closed"
        return "place:Merged"
    elseif evname == "pull_request"
        return "place:PROpen"
    elseif evname in ["check_run", "check_suite", "workflow_run"]
        if conclusion == "success"
            return "place:CIPassed"
        elseif conclusion != ""
            return "place:CIFailed"
        elseif status in ["queued", "in_progress", "requested"]
            return "place:CIRunning"
        end
    elseif evname == "release"
        return "place:Released"
    elseif occursin("alert", evname)
        return "place:SecurityAlertOpen"
    end
    return "place:CleanMain"
end
