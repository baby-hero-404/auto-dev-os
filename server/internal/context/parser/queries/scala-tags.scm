; Adapted from references/aider/aider/queries, verified against the actual grammar.

(class_definition name: (identifier) @name.definition.class) @definition.class
(trait_definition name: (identifier) @name.definition.interface) @definition.interface
(object_definition name: (identifier) @name.definition.object) @definition.object
(function_definition name: (identifier) @name.definition.function) @definition.function
(function_declaration name: (identifier) @name.definition.function) @definition.function
(enum_definition name: (identifier) @name.definition.enum) @definition.enum
(call_expression function: (identifier) @name.reference.call)
