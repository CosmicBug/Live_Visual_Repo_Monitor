function asdict(x)
    if x === nothing
        return Dict{String,Any}()
    elseif x isa Dict
        return Dict{String,Any}(String(k)=>v for (k,v) in x)
    else
        try
            return Dict{String,Any}(String(k)=>v for (k,v) in pairs(x))
        catch
            return Dict{String,Any}()
        end
    end
end

function getd(d, key::String, default=nothing)
    d = asdict(d)
    return haskey(d, key) ? d[key] : default
end

function dig(d, keys::Vector{String}, default=nothing)
    cur = d
    for k in keys
        cur = asdict(cur)
        if !haskey(cur, k)
            return default
        end
        cur = cur[k]
    end
    return cur
end

function vecdict(x)
    if x === nothing
        return Vector{Dict{String,Any}}()
    end
    return [asdict(v) for v in x]
end

function label_from_path(path::AbstractString)
    isempty(path) && return ""
    parts = split(path, "/")
    return String(parts[end])
end

function safe_id(prefix::String, value)
    return prefix * ":" * replace(string(value), " "=>"_", "\n"=>"_", "\t"=>"_")
end

function now_iso()
    return string(Dates.now(Dates.UTC)) * "Z"
end

function int_or(default::Int, x)
    x === nothing && return default
    try
        return Int(x)
    catch
        return default
    end
end
