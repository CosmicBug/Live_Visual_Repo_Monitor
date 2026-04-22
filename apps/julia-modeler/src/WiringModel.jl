function build_wiring_model(snapshot::Dict{String,Any}, diff::Dict{String,Any})
    repo = asdict(getd(snapshot, "repo", Dict()))
    repo_id = int_or(0, getd(repo, "id", 0))
    repo_name = getd(repo, "full_name", getd(repo, "name", "repository"))
    tree = asdict(getd(snapshot, "tree", Dict()))
    files = vecdict(getd(tree, "files", []))
    dirs = vecdict(getd(tree, "directories", []))
    dependencies = vecdict(getd(snapshot, "dependencies", []))
    workflows = vecdict(getd(snapshot, "workflows", []))

    status_by_file = Dict{String,String}()
    for f in vecdict(getd(diff, "added_files", [])); status_by_file[getd(f,"path","")] = "added"; end
    for f in vecdict(getd(diff, "modified_files", [])); status_by_file[getd(f,"path","")] = "modified"; end
    for f in vecdict(getd(diff, "removed_files", [])); status_by_file[getd(f,"path","")] = "removed"; end

    nodes = Vector{Dict{String,Any}}()
    edges = Vector{Dict{String,Any}}()

    repo_node = Dict{String,Any}(
        "id" => safe_id("repo", repo_id),
        "label" => repo_name,
        "kind" => "Repository",
        "status" => isempty(status_by_file) ? "stable" : "changed",
        "metadata" => repo,
    )
    push!(nodes, repo_node)

    dir_paths = Set{String}()
    for d in dirs
        p = String(getd(d, "path", ""))
        isempty(p) && continue
        push!(dir_paths, p)
        parent_path = String(getd(d, "parent", ""))
        parent_id = isempty(parent_path) ? repo_node["id"] : safe_id("dir", parent_path)
        push!(nodes, Dict{String,Any}(
            "id" => safe_id("dir", p),
            "label" => label_from_path(p),
            "kind" => "Directory",
            "parent" => parent_id,
            "status" => "stable",
            "metadata" => d,
        ))
        push!(edges, Dict{String,Any}(
            "id" => "contains:" * string(parent_id) * "->" * safe_id("dir", p),
            "source" => string(parent_id),
            "target" => safe_id("dir", p),
            "kind" => "contains",
            "status" => "stable",
        ))
    end

    file_paths = Set{String}()
    for f in files
        p = String(getd(f, "path", ""))
        isempty(p) && continue
        push!(file_paths, p)
        dir = join(split(p, "/")[1:end-1], "/")
        parent_id = isempty(dir) ? repo_node["id"] : safe_id("dir", dir)
        status = get(status_by_file, p, "stable")
        push!(nodes, Dict{String,Any}(
            "id" => safe_id("file", p),
            "label" => label_from_path(p),
            "kind" => getd(f, "kind", "File") == "workflow" ? "WorkflowFile" : "File",
            "parent" => parent_id,
            "status" => status,
            "metadata" => f,
        ))
        push!(edges, Dict{String,Any}(
            "id" => "contains:" * string(parent_id) * "->" * safe_id("file", p),
            "source" => string(parent_id),
            "target" => safe_id("file", p),
            "kind" => "contains",
            "status" => status,
        ))
    end

    for wf in workflows
        p = String(getd(wf, "path", ""))
        isempty(p) && continue
        id = safe_id("workflow", p)
        if !(p in file_paths)
            push!(nodes, Dict{String,Any}(
                "id" => id,
                "label" => getd(wf, "name", label_from_path(p)),
                "kind" => "Workflow",
                "parent" => safe_id("dir", ".github/workflows"),
                "status" => "stable",
                "metadata" => wf,
            ))
        end
    end

    external_nodes = Set{String}()
    for dep in dependencies
        source = String(getd(dep, "source", ""))
        target = String(getd(dep, "target", ""))
        kind = String(getd(dep, "kind", "dependency"))
        if isempty(source) || isempty(target)
            continue
        end
        src_id = startswith(source, ".github/workflows/") ? safe_id("workflow", source) : safe_id("file", source)
        tgt_id = target in file_paths ? safe_id("file", target) : safe_id("external", target)
        if !(target in file_paths) && !(tgt_id in external_nodes)
            push!(external_nodes, tgt_id)
            push!(nodes, Dict{String,Any}(
                "id" => tgt_id,
                "label" => target,
                "kind" => "ExternalDependency",
                "status" => "stable",
                "metadata" => Dict("target"=>target, "scope"=>getd(dep,"scope","")),
            ))
        end
        push!(edges, Dict{String,Any}(
            "id" => kind * ":" * src_id * "->" * tgt_id,
            "source" => src_id,
            "target" => tgt_id,
            "kind" => kind,
            "status" => haskey(status_by_file, source) ? "changed" : "stable",
            "metadata" => dep,
        ))
    end

    return Dict{String,Any}(
        "kind" => "CatlabCompatibleWiringDiagram",
        "repo" => repo,
        "nodes" => nodes,
        "edges" => edges,
        "ports" => Vector{Any}(),
        "wires" => [e for e in edges if getd(e, "kind", "") != "contains"],
        "metadata" => Dict(
            "ecosystem" => "AlgebraicJulia",
            "catlab_package" => "Catlab.jl",
            "interpretation" => "Repository directories/files are boxes; dependency/import/build relations are wires."
        )
    )
end
