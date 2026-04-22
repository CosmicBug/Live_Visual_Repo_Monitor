function compile_model(req::Dict{String,Any})
    snapshot = asdict(getd(req, "snapshot", Dict()))
    previous_model = asdict(getd(req, "previous_model", Dict()))
    diff = asdict(getd(req, "diff", Dict()))
    event = asdict(getd(req, "event", Dict()))
    context = asdict(getd(req, "context", Dict()))

    version = int_or(0, getd(context, "model_version", 0))
    if version == 0
        version = int_or(0, getd(previous_model, "model_version", 0)) + 1
    end

    if isempty(snapshot) && !isempty(previous_model)
        wiring = model_to_wiring(previous_model)
    elseif isempty(snapshot)
        wiring = empty_wiring_from_event(event)
    else
        wiring = build_wiring_model(snapshot, diff)
    end

    petri = build_petri_model(event, snapshot)
    viz = export_viz_model(wiring, petri, event, version)
    patch = diff_viz(previous_model, viz, event, version)

    return Dict{String,Any}(
        "model_version" => version,
        "catlab_model" => wiring,
        "petri_model" => petri,
        "viz_model" => viz,
        "patch" => patch,
        "warnings" => Vector{String}(),
    )
end

function model_to_wiring(previous_model::Dict{String,Any})
    nodes = vecdict(getd(previous_model, "nodes", []))
    edges = vecdict(getd(previous_model, "edges", []))
    topo_nodes = [n for n in nodes if !(String(getd(n,"kind","")) in ["PetriPlace", "PetriTransition", "PetriToken", "GitHubEvent", "PullRequest", "CIStatus"])]
    topo_edges = [e for e in edges if !(String(getd(e,"kind","")) in ["petri-arc", "token-at", "observed-event", "pr-event", "ci-event"])]
    return Dict{String,Any}(
        "kind" => "CatlabCompatibleWiringDiagram",
        "repo" => asdict(getd(previous_model, "repo", Dict())),
        "nodes" => topo_nodes,
        "edges" => topo_edges,
        "ports" => Vector{Any}(),
        "wires" => [e for e in topo_edges if getd(e, "kind", "") != "contains"],
        "metadata" => Dict("ecosystem"=>"AlgebraicJulia", "source"=>"previous_model")
    )
end

function empty_wiring_from_event(event::Dict{String,Any})
    repo = asdict(getd(event, "repository", Dict()))
    repo_id = int_or(0, getd(repo, "id", 0))
    repo_name = getd(repo, "full_name", getd(repo, "name", "repository"))
    return Dict{String,Any}(
        "kind" => "CatlabCompatibleWiringDiagram",
        "repo" => repo,
        "nodes" => [Dict("id"=>safe_id("repo", repo_id), "label"=>repo_name, "kind"=>"Repository", "status"=>"changed", "metadata"=>repo)],
        "edges" => Vector{Dict{String,Any}}(),
        "ports" => Vector{Any}(),
        "wires" => Vector{Any}(),
        "metadata" => Dict("ecosystem"=>"AlgebraicJulia", "source"=>"event_only")
    )
end

function diff_viz(previous::Dict{String,Any}, next::Dict{String,Any}, event::Dict{String,Any}, version::Int)
    prev_version = int_or(0, getd(previous, "model_version", 0))
    changes = Vector{Dict{String,Any}}()
    prev_nodes = Dict{String,Dict{String,Any}}()
    prev_edges = Dict{String,Dict{String,Any}}()
    for n in vecdict(getd(previous, "nodes", [])); prev_nodes[String(getd(n,"id",""))] = n; end
    for e in vecdict(getd(previous, "edges", [])); prev_edges[String(getd(e,"id",""))] = e; end

    next_nodes = Dict{String,Dict{String,Any}}()
    next_edges = Dict{String,Dict{String,Any}}()
    for n in vecdict(getd(next, "nodes", [])); next_nodes[String(getd(n,"id",""))] = n; end
    for e in vecdict(getd(next, "edges", [])); next_edges[String(getd(e,"id",""))] = e; end

    for (id, n) in next_nodes
        if !haskey(prev_nodes, id)
            push!(changes, Dict("op"=>"add_node", "id"=>id, "kind"=>getd(n,"kind",""), "node"=>n, "animation"=>animation_for_status(getd(n,"status","stable"))))
        else
            status_changed = getd(prev_nodes[id], "status", "") != getd(n, "status", "")
            metadata_changed = JSON3.write(getd(prev_nodes[id], "metadata", Dict())) != JSON3.write(getd(n, "metadata", Dict()))
            if status_changed || metadata_changed
                push!(changes, Dict("op"=>"update_node", "id"=>id, "kind"=>getd(n,"kind",""), "set"=>Dict("status"=>getd(n,"status","stable"), "metadata"=>getd(n,"metadata",Dict())), "animation"=>animation_for_status(getd(n,"status","stable"))))
            end
        end
    end
    for (id, n) in prev_nodes
        if !haskey(next_nodes, id)
            push!(changes, Dict("op"=>"remove_node", "id"=>id, "kind"=>getd(n,"kind",""), "animation"=>"fade-out"))
        end
    end

    for (id, e) in next_edges
        if !haskey(prev_edges, id)
            push!(changes, Dict("op"=>"add_edge", "id"=>id, "kind"=>getd(e,"kind",""), "edge"=>e, "animation"=>animation_for_status(getd(e,"status","stable"))))
        else
            if getd(prev_edges[id], "status", "") != getd(e, "status", "")
                push!(changes, Dict("op"=>"update_edge", "id"=>id, "kind"=>getd(e,"kind",""), "set"=>Dict("status"=>getd(e,"status","stable"), "metadata"=>getd(e,"metadata",Dict())), "animation"=>animation_for_status(getd(e,"status","stable"))))
            end
        end
    end
    for (id, e) in prev_edges
        if !haskey(next_edges, id)
            push!(changes, Dict("op"=>"remove_edge", "id"=>id, "kind"=>getd(e,"kind",""), "animation"=>"fade-out"))
        end
    end

    return Dict{String,Any}(
        "type" => "VizPatch",
        "repo_id" => int_or(0, dig(next, ["repo", "id"], dig(event, ["repository", "id"], 0))),
        "patch_id" => string(uuid4()),
        "from_version" => prev_version,
        "to_version" => version,
        "model_version" => version,
        "changes" => changes,
        "event" => getd(event, "event", ""),
        "created_at" => now_iso(),
    )
end

function animation_for_status(status)
    s = String(status)
    if s in ["added", "active", "changed"]
        return "pulse"
    elseif s in ["modified"]
        return "glow"
    elseif s in ["failed"]
        return "blink"
    elseif s in ["passed"]
        return "flash"
    elseif s in ["removed"]
        return "fade-out"
    end
    return "none"
end
