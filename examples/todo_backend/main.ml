open Lwt.Syntax

module Runtime = Sqlc_ocaml_runtime_lwt

let database_url =
  Sys.getenv_opt "DATABASE_URL"
  |> Option.value ~default:"postgresql://todo:todo@localhost:5439/todobackend"

let public_todos_url =
  Sys.getenv_opt "PUBLIC_TODOS_URL"
  |> Option.value ~default:"http://localhost:8080/todos"

let pool = Runtime.Database.connect_uri_exn database_url

let with_database operation = Runtime.Database.use pool operation

let cors_headers =
  [ ("Access-Control-Allow-Origin", "*");
    ("Access-Control-Allow-Headers", "Accept, Content-Type");
    ("Access-Control-Allow-Methods", "GET, HEAD, POST, DELETE, OPTIONS, PUT, PATCH") ]

let cors inner request =
  let* response = inner request in
  List.iter (fun (name, value) -> Dream.add_header response name value) cors_headers;
  Lwt.return response

let json ?(status = `OK) value =
  Dream.json ~status (Yojson.Safe.to_string value)

let error ?(status = `Bad_Request) message =
  json ~status (`Assoc [ ("error", `String message) ])

let todo_json ~id ~title ~completed ~order =
  `Assoc
    [ ("title", `String title);
      ("completed", `Bool completed);
      ("order", `Int (Int32.to_int order));
      ("url", `String (Printf.sprintf "%s/%Ld" public_todos_url id)) ]

let todo_row_json (row : Queries.todo_row) =
  todo_json ~id:row.id ~title:row.title ~completed:row.completed ~order:row.order

let int64_param request =
  match Int64.of_string_opt (Dream.param request "id") with
  | Some id -> Ok id
  | None -> Error "invalid todo id"

let object_body request =
  let* body = Dream.body request in
  try
    match Yojson.Safe.from_string body with
    | `Assoc fields -> Lwt.return (Ok fields)
    | _ -> Lwt.return (Error "request body must be a JSON object")
  with Yojson.Json_error message -> Lwt.return (Error message)

let field name fields = List.assoc_opt name fields

let title_field fields =
  match field "title" fields with
  | Some (`String value) -> Ok value
  | Some _ -> Error "title must be a string"
  | None -> Error "title is required"

let bool_field ~default name fields =
  match field name fields with
  | Some (`Bool value) -> Ok value
  | Some _ -> Error (name ^ " must be a boolean")
  | None -> Ok default

let order_field ~default fields =
  match field "order" fields with
  | Some (`Int value) -> Ok (Int32.of_int value)
  | Some (`Intlit value) ->
      (match Int32.of_string_opt value with Some value -> Ok value | None -> Error "order must be an integer")
  | Some _ -> Error "order must be an integer"
  | None -> Ok default

let optional_title_field fields =
  match field "title" fields with
  | Some (`String value) -> Ok (Some value)
  | Some _ -> Error "title must be a string"
  | None -> Ok None

let optional_bool_field name fields =
  match field name fields with
  | Some (`Bool value) -> Ok (Some value)
  | Some _ -> Error (name ^ " must be a boolean")
  | None -> Ok None

let optional_order_field fields =
  match field "order" fields with
  | Some (`Int value) -> Ok (Some (Int32.of_int value))
  | Some (`Intlit value) ->
      (match Int32.of_string_opt value with Some value -> Ok (Some value) | None -> Error "order must be an integer")
  | Some _ -> Error "order must be an integer"
  | None -> Ok None

let get_all _request =
  let* result = with_database (fun db -> Queries.ListTodos.execute db ()) in
  match result with
  | Ok todos -> json (`List (List.map todo_row_json todos))
  | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error)

let get_one request =
  match int64_param request with
  | Error message -> error message
  | Ok id ->
      let* result = with_database (fun db -> Queries.GetTodo.execute db { id }) in
      (match result with
      | Ok (todo :: _) -> json (todo_row_json todo)
      | Ok [] -> error ~status:`Not_Found "todo not found"
      | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error))

let create request =
  let* parsed = object_body request in
  match parsed with
  | Error message -> error message
  | Ok fields ->
      (match (title_field fields, bool_field ~default:false "completed" fields, order_field ~default:0l fields) with
      | Ok title, Ok completed, Ok order ->
          let params : Queries.CreateTodo.params = { title; completed; todo_order = order } in
          let* result = with_database (fun db -> Queries.CreateTodo.execute db params) in
          (match result with
          | Ok todo -> json ~status:`Created (todo_row_json todo)
          | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error))
      | Error message, _, _ | _, Error message, _ | _, _, Error message -> error message)

let patch request =
  match int64_param request with
  | Error message -> error message
  | Ok id ->
      let* parsed = object_body request in
      (match parsed with
      | Error message -> error message
      | Ok fields ->
          (match (optional_title_field fields, optional_bool_field "completed" fields, optional_order_field fields) with
          | Ok title, Ok completed, Ok todo_order ->
              let params : Queries.PatchTodo.params = { id; title; completed; todo_order } in
              let* result = with_database (fun db -> Queries.PatchTodo.execute db params) in
              (match result with
              | Ok (updated :: _) -> json (todo_row_json updated)
              | Ok [] -> error ~status:`Not_Found "todo not found"
              | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error))
          | Error message, _, _ | _, Error message, _ | _, _, Error message -> error message))

let delete_one request =
  match int64_param request with
  | Error message -> error message
  | Ok id ->
      let* result = with_database (fun db -> Queries.DeleteTodo.execute db { id }) in
      (match result with
      | Ok () -> Dream.empty `No_Content
      | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error))

let delete_all _request =
  let* result = with_database (fun db -> Queries.DeleteAllTodos.execute db ()) in
  match result with
  | Ok () -> Dream.empty `No_Content
  | Error db_error -> error ~status:`Internal_Server_Error (Caqti_error.show db_error)

let options _request = Dream.empty `No_Content

let () =
  Dream.run ~interface:"0.0.0.0" ~port:8080
  @@ Dream.logger
  @@ cors
  @@ Dream.router
       [ Dream.options "/**" options;
         Dream.get "/todos" get_all;
         Dream.post "/todos" create;
         Dream.delete "/todos" delete_all;
         Dream.get "/todos/:id" get_one;
         Dream.patch "/todos/:id" patch;
         Dream.delete "/todos/:id" delete_one ]
