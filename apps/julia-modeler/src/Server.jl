function json_response(status::Int, payload)
    return HTTP.Response(status, ["Content-Type" => "application/json"], JSON3.write(payload))
end

function handle_request(req::HTTP.Request)
    try
        path = String(HTTP.URI(req.target).path)
        if req.method == "GET" && path == "/health"
            return json_response(200, Dict("status"=>"ok"))
        elseif req.method == "GET" && path == "/healthz"
            return json_response(200, Dict("status"=>"ok"))
        elseif req.method == "POST" && path == "/compile"
            body = String(req.body)
            payload = JSON3.read(body, Dict{String,Any})
            result = compile_model(payload)
            return json_response(200, result)
        else
            return json_response(404, Dict("error"=>"not found"))
        end
    catch err
        return json_response(500, Dict("error"=>sprint(showerror, err)))
    end
end

function run_server(host::AbstractString="0.0.0.0", port::Integer=8090)
    @info "Julia modeler listening" host=host port=port
    HTTP.serve(handle_request, host, Int(port); verbose=false)
end
