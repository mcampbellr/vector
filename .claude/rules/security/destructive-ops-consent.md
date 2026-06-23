# Security — Consentimiento para operaciones destructivas

> Aplica a: cualquier operación que escriba, mueva, reorganice o elimine archivos en el repo
> **del usuario**. Crítico — es la garantía de seguridad central de Vector.

Vector es agnóstico al código del usuario y opera sobre repos ajenos. Toda mutación de ese
repo es potencialmente destructiva y requiere salvaguardas explícitas.

## Reglas

1. **Backup antes de reorganizar**: antes de transformar la estructura del repo al formato que
   Vector requiere (p. ej. durante `/vector init`), pedir **permiso EXPLÍCITO** al usuario y
   crear un **backup del estado actual**.
2. **Respetar `.gitignore`**: el backup/reorganización ignora lo que el stack indique ignorar
   (`.gitignore` o equivalente), por stack.
3. **Nada de mutaciones silenciosas**: ninguna escritura sobre el repo del usuario ocurre sin
   confirmación previa. Las operaciones de solo lectura (detección) no requieren permiso, pero
   no deben modificar nada.
4. **Reversibilidad**: preferir operaciones reversibles; si algo es difícil de revertir,
   confirmarlo aparte aunque ya exista permiso general (alineado al global del usuario).
5. **Mirar antes de borrar/sobrescribir**: si el contenido contradice cómo fue descrito, o
   Vector no lo creó, surfacearlo en vez de proceder.

## Alcance

- Esto cubre el **repo del usuario**, no el repo de Vector ni el JSON de estado de Vector
  (ese se gobierna por `architecture/state-model.md` y `workflows/state-sync-discipline.md`).

> Estado: pendiente — mecanismo concreto de backup (ubicación, formato, restore) se define con
> la implementación de `/vector init`.
