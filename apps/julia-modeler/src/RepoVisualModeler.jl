module RepoVisualModeler

using Dates
using UUIDs
using JSON3
using HTTP

# These imports pin the modeler to the AlgebraicJulia ecosystem. The service emits
# a stable Catlab-compatible wiring model and AlgebraicPetri-compatible Petri model
# as JSON so the frontend and Go services are insulated from package API churn.
import Catlab
import AlgebraicPetri

include("Utils.jl")
include("WiringModel.jl")
include("PetriModel.jl")
include("VizExport.jl")
include("Diff.jl")
include("Server.jl")

export compile_model, run_server

end
