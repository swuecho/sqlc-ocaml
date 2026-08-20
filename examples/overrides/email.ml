type t = string

let of_string value =
  if String.contains value '@' then Ok value
  else Error (Printf.sprintf "invalid email address: %s" value)

let to_string value = value

let codec =
  Caqti_type.custom
    ~encode:(fun value -> Ok (to_string value))
    ~decode:of_string
    Caqti_type.string
