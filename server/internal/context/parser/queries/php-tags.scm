; Adapted from references/aider/aider/queries, verified against the actual grammar.

(class_declaration name: (name) @name.definition.class) @definition.class
(function_definition name: (name) @name.definition.function) @definition.function
(method_declaration name: (name) @name.definition.method) @definition.method
(function_call_expression function: (name) @name.reference.call)
(member_call_expression name: (name) @name.reference.call)
(scoped_call_expression name: (name) @name.reference.call)
(object_creation_expression (qualified_name (name) @name.reference.call))
