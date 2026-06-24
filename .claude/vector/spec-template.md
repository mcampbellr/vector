# Spec Template — Vector

Plantilla canónica que el comando `/vector:raw` debe usar al escribir cualquier spec nuevo. Las 20 secciones son obligatorias y deben aparecer en este orden. No se eliminan secciones: si una sección no aplica, se deja con la nota `No aplica — <razón>` (no se omite).

Cuando `SPEC_LANGUAGE = english`, se traducen los encabezados al inglés (`## 1. Goal`, `## 2. Scope`, …) y todo el cuerpo se escribe en inglés, pero la estructura, profundidad y orden de las 20 secciones se mantiene idéntica. Slugs, rutas, identificadores de código y artefactos de git permanecen en kebab-case inglés.

Las tablas, checklists y placeholders `[...]` deben sobrevivir en la salida final: el agente reemplaza cada `[...]` por contenido concreto verificado contra el repo. Si no hay evidencia, deja `TBD — ver Open questions` y registra la duda. Nunca inventar versiones, rutas, ni endpoints.

---

# Spec: [Nombre de la feature/fase]

## 1. Objetivo

Construir [descripción funcional clara de lo que se debe construir].

Esta feature permite que [tipo de usuario] pueda [acción principal] para [resultado esperado].

## 2. Alcance

### Incluido en esta fase

- [Funcionalidad 1]
- [Funcionalidad 2]
- [Pantalla / módulo / flujo específico]
- [Validaciones incluidas]
- [Integraciones incluidas]

### Fuera de scope

- [Funcionalidad explícitamente excluida]
- [Pantalla o flujo que no debe tocarse]
- [Automatizaciones futuras no incluidas]
- [Optimizaciones no requeridas]
- [Casos que se resolverán en otra fase]

El agente no debe implementar nada fuera del alcance definido arriba, aunque parezca relacionado.

---

## 3. Tecnologías y convenciones del proyecto

### Stack

- Framework: [Ej. Flutter / React Native / Next.js / NestJS]
- Lenguaje: [Ej. Dart / TypeScript]
- Package manager: [Ej. pnpm / yarn / npm]
- UI library: [Ej. Chakra UI v3 / NativeBase / Material]
- State management: [Ej. BLoC / Riverpod / Zustand / React Query]
- API client: [Ej. Dio / Axios / Fetch wrapper interno]
- Forms: [Ej. React Hook Form / Formik / Flutter Form]
- Validation: [Ej. Zod / Yup / class-validator]
- Testing: [Ej. Jest / Vitest / Flutter test]

### Versiones relevantes

- [Framework]: [versión]
- [Librería principal]: [versión]
- [SDK externo]: [versión]

No usar librerías, APIs, flags o patrones que no estén documentados oficialmente o que no estén ya presentes en el proyecto, salvo que este spec lo autorice explícitamente.

### Patrones existentes a respetar

- [Patrón 1]
- [Patrón 2]
- [Convención de nombres]
- [Convención de imports]
- [Convención de carpetas]
- [Convención de errores]
- [Convención de traducciones/i18n]

---

## 4. Dependencias previas

Antes de iniciar esta fase debe existir o estar completado:

- [ ] [Módulo / endpoint / pantalla / componente previo]
- [ ] [Archivo de contrato API]
- [ ] [Variables de entorno]
- [ ] [Diseño aprobado]
- [ ] [Tipos/interfaces existentes]
- [ ] [Servicio base ya implementado]
- [ ] [Feature flag, permiso o configuración requerida]

Si alguna dependencia no existe, el agente debe detenerse y reportar exactamente qué falta. No debe inventar contratos, rutas ni estructuras.

---

## 5. Arquitectura

### Patrón a usar

Usar el patrón: [BLoC / MVVM / Clean Architecture / Feature-first / etc.]

### Capas afectadas

Esta fase puede tocar únicamente las siguientes capas:

- presentation: [sí/no + descripción]
- application/use-cases: [sí/no + descripción]
- domain: [sí/no + descripción]
- data/infrastructure: [sí/no + descripción]
- shared/common: [sí/no + descripción]

### Flujo esperado

1. Usuario ejecuta [acción].
2. UI dispara [evento / action / mutation].
3. Estado pasa a [loading state].
4. Se llama a [servicio / repository / API client].
5. La respuesta se transforma usando [mapper / DTO / schema].
6. El estado cambia a [success state] o [error state].
7. La UI muestra [resultado visible].

### Ubicación de archivos nuevos

Los archivos nuevos deben ubicarse siguiendo este patrón:

```txt
[feature-folder]/
  presentation/
  application/
  domain/
  data/
```

No crear carpetas nuevas si ya existe una convención equivalente en el proyecto.

---

## 6. Archivos a crear o modificar

La lista debe ser exacta. No modificar archivos fuera de esta sección sin justificarlo.

| Ruta | Acción | Propósito | Ejemplo del proyecto a seguir |
|---|---|---|---|
| path/to/file.ext | NUEVO | [Qué contiene] | path/to/existing-example.ext |
| path/to/file.ext | MODIFICAR | [Qué cambio hacer] | path/to/existing-example.ext |
| path/to/file.ext | NUEVO | [Qué contiene] | path/to/existing-example.ext |

### Detalle por archivo

#### path/to/file.ext

Acción: NUEVO

Debe implementar:

- [Responsabilidad 1]
- [Responsabilidad 2]

Debe seguir como referencia:

- path/to/existing-example.ext

No debe incluir:

- [Cosa que no debe mezclarse en este archivo]

#### path/to/existing-file.ext

Acción: MODIFICAR

Cambios requeridos:

- [Cambio exacto 1]
- [Cambio exacto 2]

Restricciones:

- No cambiar [comportamiento existente].
- No refactorizar partes no relacionadas.
- Mantener compatibilidad con [flujo existente].

---

## 7. API Contract

El contrato exacto de API está definido en:

```txt
docs/api-contract.md
```

El agente debe usar ese archivo como única fuente de verdad para:

- Endpoints
- Métodos HTTP
- Headers requeridos
- Request body
- Query params
- Response body
- Códigos de error
- Mensajes de error
- Estados de loading/success/error derivados de la API

No inferir campos adicionales ni cambiar nombres de propiedades.

### Endpoints involucrados

- [METHOD] /path/to/endpoint
- [METHOD] /path/to/endpoint/:id

Para más detalle, ver docs/api-contract.md.

---

## 8. Criterios de éxito

La implementación se considera correcta cuando:

- [ ] [Comportamiento funcional verificable]
- [ ] [Endpoint llamado correctamente]
- [ ] [Estado de éxito mostrado correctamente]
- [ ] [Estado de error mostrado correctamente]
- [ ] [Validaciones funcionando]
- [ ] [No se rompe flujo existente]
- [ ] [No hay errores de TypeScript / Dart analyzer / linter]
- [ ] [Tests nuevos pasan]
- [ ] [Tests existentes pasan]

### Tests requeridos

Agregar o actualizar tests para:

- [ ] Caso exitoso
- [ ] Error de validación
- [ ] Error de API
- [ ] Estado loading
- [ ] Edge case relevante
- [ ] Transformación de datos, si aplica

### Comandos de verificación

Ejecutar:

```bash
[comando de lint]
[comando de typecheck]
[comando de tests]
[comando de build]
```

La fase no está completa si alguno de estos comandos falla.

---

## 9. Criterios de UX

La UI debe cumplir exactamente con estos comportamientos:

### Loading

- Al enviar el formulario, el botón principal debe mostrar [spinner/loading indicator].
- El botón debe quedar deshabilitado mientras la request está activa.
- No debe permitirse doble submit.
- Si hay carga inicial, mostrar [skeleton / spinner / placeholder].

### Formularios

- Los errores de validación deben mostrarse inline debajo de cada campo.
- El mensaje de error debe desaparecer cuando el usuario corrige el campo.
- Al tocar fuera de un TextField/Input, el teclado debe cerrarse.
- Campos obligatorios deben indicarse con [criterio visual].
- El submit debe validar todos los campos antes de llamar a la API.

### Passwords

- Los campos password deben tener toggle de visibilidad.
- El ícono debe reflejar correctamente el estado visible/oculto.
- El valor del campo no debe perderse al alternar visibilidad.

### Errores

- Errores de campo: mostrarlos inline.
- Errores generales: mostrarlos en [toast / alert / banner].
- Errores de red: mostrar mensaje claro y permitir reintentar.
- Timeouts: mostrar mensaje claro y no dejar la UI bloqueada.

### Navegación

- En éxito, navegar a [ruta/pantalla destino].
- En cancelación, volver a [ruta/pantalla anterior].
- No usar navegación implícita no definida en este spec.

### Accesibilidad

- Botones deben tener labels accesibles.
- Inputs deben tener labels visibles o accesibles.
- Estados de error deben ser legibles por tecnologías asistivas si el framework lo permite.

---

## 10. Decisiones tomadas

Estas decisiones ya están tomadas y el agente no debe cuestionarlas ni cambiarlas:

- [Decisión 1]
- [Decisión 2]
- [Decisión 3]
- [Patrón elegido]
- [Librería elegida]
- [Nombre de archivos/carpetas elegido]
- [Contrato API elegido]
- [Flujo UX elegido]

Si el agente detecta una alternativa aparentemente mejor, debe reportarla como observación, pero no implementarla.

---

## 11. Edge cases

La implementación debe manejar explícitamente:

### Datos inválidos

- [Caso de dato inválido]
- [Campo requerido vacío]
- [Formato inválido]
- [Valores fuera de rango]

Comportamiento esperado:

- Mostrar error inline.
- No llamar a la API si la validación local falla.

### API errors

- 400: [comportamiento esperado]
- 401: [comportamiento esperado]
- 403: [comportamiento esperado]
- 404: [comportamiento esperado]
- 409: [comportamiento esperado]
- 422: [comportamiento esperado]
- 429: [comportamiento esperado]
- 500: [comportamiento esperado]

### Sin conexión

Comportamiento esperado:

- Mostrar [mensaje específico].
- Mantener los datos ingresados por el usuario.
- Permitir reintentar.

### Timeout

Comportamiento esperado:

- Detener loading.
- Mostrar [mensaje específico].
- Permitir reintentar.

### Respuesta vacía o inesperada

Comportamiento esperado:

- No romper la UI.
- Mostrar fallback definido.
- Registrar/loggear el error si el proyecto ya tiene mecanismo de logging.

### Doble submit

Comportamiento esperado:

- Bloquear submits simultáneos.
- Ejecutar una sola request.

---

## 12. Estados de UI requeridos

La pantalla/componente debe contemplar estos estados:

- idle
- loading
- success
- error
- empty, si aplica
- disabled, si aplica
- offline, si aplica

Para cada estado:

| Estado | Qué se muestra | Qué puede hacer el usuario |
|---|---|---|
| idle | [UI inicial] | [acciones disponibles] |
| loading | [spinner/skeleton] | [acciones bloqueadas/permitidas] |
| success | [resultado] | [siguiente acción] |
| error | [mensaje de error] | [reintentar/cancelar] |
| empty | [mensaje vacío] | [acción alternativa] |
| offline | [mensaje sin conexión] | [reintentar] |

---

## 13. Validaciones

### Validaciones de cliente

| Campo | Regla | Mensaje |
|---|---|---|
| [campo] | [regla] | [mensaje exacto] |
| [campo] | [regla] | [mensaje exacto] |

### Validaciones de servidor

Las validaciones del servidor están definidas en docs/api-contract.md.

Si el servidor devuelve errores por campo, deben mapearse al campo correspondiente en la UI.

---

## 14. Seguridad y permisos

- No exponer secrets, tokens privados o claves sensibles en frontend/mobile.
- No guardar información sensible en logs.
- No imprimir payloads sensibles en consola.
- Validar permisos antes de mostrar acciones restringidas, si aplica.
- Manejar 401/403 según el flujo definido en el proyecto.

---

## 15. Observabilidad y logging

Usar únicamente el mecanismo de logging existente en el proyecto.

Registrar:

- Error inesperado de API.
- Error de parsing/mapping.
- Timeout.
- Caso no recuperable.

No registrar:

- Passwords.
- Tokens.
- Información personal sensible.
- Payloads completos si contienen datos privados.

---

## 16. i18n / textos visibles

Todos los textos visibles deben venir de [archivo/sistema de traducciones del proyecto].

No hardcodear textos visibles en componentes, salvo que el proyecto ya lo permita explícitamente.

Textos requeridos:

| Key | Texto |
|---|---|
| feature.title | [Texto] |
| feature.submit | [Texto] |
| feature.loading | [Texto] |
| feature.success | [Texto] |
| feature.error.generic | [Texto] |
| feature.error.offline | [Texto] |

---

## 17. Performance

- Evitar renders innecesarios.
- No hacer llamadas de API repetidas sin necesidad.
- Cancelar/debouncer requests si el flujo lo requiere.
- No bloquear el hilo principal con transformaciones pesadas.
- Respetar patrones existentes de caching si el proyecto los usa.

---

## 18. Restricciones

El agente no debe:

- Cambiar contratos de API.
- Crear nuevas abstracciones globales sin autorización.
- Instalar nuevas dependencias sin que estén listadas en este spec.
- Refactorizar código no relacionado.
- Cambiar estilos globales.
- Cambiar navegación global.
- Modificar estructura de carpetas fuera del scope.
- Inventar UX no definida.
- Ignorar errores de lint/typecheck/tests.
- Usar APIs no documentadas oficialmente.

---

## 19. Entregables

Al finalizar, deben quedar:

- [ ] Código implementado.
- [ ] Tests agregados/actualizados.
- [ ] Traducciones agregadas/actualizadas.
- [ ] Tipos/interfaces agregados/actualizados.
- [ ] Integración con API completa.
- [ ] Estados de UX implementados.
- [ ] Edge cases cubiertos.
- [ ] Documentación actualizada, si aplica.

---

## 20. Checklist final para el agente

Antes de entregar, verificar:

- [ ] Leí este spec completo.
- [ ] Revisé docs/api-contract.md.
- [ ] Confirmé que todas las dependencias previas existen.
- [ ] Solo modifiqué archivos listados o justifiqué cualquier excepción.
- [ ] Seguí los ejemplos reales del proyecto.
- [ ] Implementé todos los estados de UI requeridos.
- [ ] Implementé todos los edge cases definidos.
- [ ] No agregué dependencias no autorizadas.
- [ ] No cambié decisiones tomadas.
- [ ] Ejecuté lint.
- [ ] Ejecuté typecheck/analyzer.
- [ ] Ejecuté tests.
- [ ] Ejecuté build si aplica.
- [ ] No dejé logs temporales.
- [ ] No dejé TODOs sin justificar.
