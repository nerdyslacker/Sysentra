//go:build linux && cgo

package flatpakplan

/*
#cgo LDFLAGS: -ldl
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <dlfcn.h>

typedef void FlatpakInstallation;
typedef void FlatpakTransaction;
typedef void FlatpakTransactionOperation;
typedef void GCancellable;
typedef unsigned long gulong;
typedef int gboolean;
typedef struct _GError { unsigned int domain; int code; char *message; } GError;
typedef struct _GList { void *data; struct _GList *next; struct _GList *prev; } GList;

static FlatpakInstallation *(*p_new_user)(GCancellable*, GError**);
static FlatpakInstallation *(*p_new_system)(GCancellable*, GError**);
static FlatpakTransaction *(*p_new_transaction)(FlatpakInstallation*, GCancellable*, GError**);
static gboolean (*p_add_install)(FlatpakTransaction*, const char*, const char*, const char**, GError**);
static gboolean (*p_add_update)(FlatpakTransaction*, const char*, const char**, const char*, GError**);
static gboolean (*p_add_uninstall)(FlatpakTransaction*, const char*, GError**);
static gboolean (*p_run)(FlatpakTransaction*, GCancellable*, GError**);
static GList *(*p_get_operations)(FlatpakTransaction*);
static const char *(*p_get_ref)(FlatpakTransactionOperation*);
static int (*p_get_type)(FlatpakTransactionOperation*);
static const char *(*p_type_string)(int);
static uint64_t (*p_download_size)(FlatpakTransactionOperation*);
static uint64_t (*p_installed_size)(FlatpakTransactionOperation*);
static gboolean (*p_requires_auth)(FlatpakTransactionOperation*);
static gulong (*p_signal_connect)(void*, const char*, void*, void*, void*, int);
static void (*p_unref)(void*);
static void (*p_error_free)(GError*);

typedef struct {
  int ready;
  int count;
  struct { char ref[512]; char action[32]; uint64_t download; uint64_t installed; int auth; } ops[256];
} Preview;

static gboolean collect_operations(FlatpakTransaction *tx, void *data) {
  Preview *preview = data;
  preview->ready = 1;
  for (GList *item = p_get_operations(tx); item && preview->count < 256; item = item->next) {
    FlatpakTransactionOperation *op = item->data;
    int i = preview->count++;
    const char *ref = p_get_ref(op);
    const char *action = p_type_string(p_get_type(op));
    if (ref) strncpy(preview->ops[i].ref, ref, sizeof(preview->ops[i].ref)-1);
    if (action) strncpy(preview->ops[i].action, action, sizeof(preview->ops[i].action)-1);
    preview->ops[i].download = p_download_size(op);
    preview->ops[i].installed = p_installed_size(op);
    preview->ops[i].auth = p_requires_auth(op);
  }
  return 0;
}

static int resolve_transaction(const char *scope, const char *remote, const char *operation, char **refs, int count, Preview *preview, char **error_message) {
  void *flatpak = dlopen("libflatpak.so.0", RTLD_NOW | RTLD_LOCAL);
  void *gobject = dlopen("libgobject-2.0.so.0", RTLD_NOW | RTLD_LOCAL);
  void *glib = dlopen("libglib-2.0.so.0", RTLD_NOW | RTLD_LOCAL);
  if (!flatpak || !gobject || !glib) { *error_message = strdup("libflatpak is not available"); return 0; }
#define LOAD(target, handle, symbol) do { *(void **)(&target) = dlsym(handle, symbol); if (!target) { *error_message = strdup("installed libflatpak is missing a required transaction API"); return 0; } } while (0)
  LOAD(p_new_user, flatpak, "flatpak_installation_new_user"); LOAD(p_new_system, flatpak, "flatpak_installation_new_system");
  LOAD(p_new_transaction, flatpak, "flatpak_transaction_new_for_installation"); LOAD(p_add_install, flatpak, "flatpak_transaction_add_install");
  LOAD(p_add_update, flatpak, "flatpak_transaction_add_update"); LOAD(p_add_uninstall, flatpak, "flatpak_transaction_add_uninstall"); LOAD(p_run, flatpak, "flatpak_transaction_run");
  LOAD(p_get_operations, flatpak, "flatpak_transaction_get_operations"); LOAD(p_get_ref, flatpak, "flatpak_transaction_operation_get_ref");
  LOAD(p_get_type, flatpak, "flatpak_transaction_operation_get_operation_type"); LOAD(p_type_string, flatpak, "flatpak_transaction_operation_type_to_string");
  LOAD(p_download_size, flatpak, "flatpak_transaction_operation_get_download_size"); LOAD(p_installed_size, flatpak, "flatpak_transaction_operation_get_installed_size");
  LOAD(p_requires_auth, flatpak, "flatpak_transaction_operation_get_requires_authentication"); LOAD(p_signal_connect, gobject, "g_signal_connect_data");
  LOAD(p_unref, gobject, "g_object_unref"); LOAD(p_error_free, glib, "g_error_free");
#undef LOAD
  GError *error = NULL;
  FlatpakInstallation *installation = strcmp(scope, "user") == 0 ? p_new_user(NULL, &error) : p_new_system(NULL, &error);
  if (!installation) goto fail;
  FlatpakTransaction *tx = p_new_transaction(installation, NULL, &error);
  if (!tx) { p_unref(installation); goto fail; }
  for (int i = 0; i < count; i++) {
    gboolean ok = strcmp(operation, "install") == 0 ? p_add_install(tx, remote, refs[i], NULL, &error) :
                  strcmp(operation, "update") == 0 ? p_add_update(tx, refs[i], NULL, NULL, &error) : p_add_uninstall(tx, refs[i], &error);
    if (!ok) { p_unref(tx); p_unref(installation); goto fail; }
  }
  p_signal_connect(tx, "ready-pre-auth", collect_operations, preview, NULL, 0);
  p_run(tx, NULL, &error);
  p_unref(tx); p_unref(installation);
  if (preview->ready) { if (error) p_error_free(error); return 1; }
fail:
  if (error && error->message) *error_message = strdup(error->message);
  if (error) p_error_free(error);
  return 0;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func resolve(scope, remote, operation string, refs []string) ([]Change, error) {
	cScope, cRemote, cOperation := C.CString(scope), C.CString(remote), C.CString(operation)
	defer C.free(unsafe.Pointer(cScope))
	defer C.free(unsafe.Pointer(cRemote))
	defer C.free(unsafe.Pointer(cOperation))
	cRefs := make([]*C.char, len(refs))
	for index, ref := range refs {
		cRefs[index] = C.CString(ref)
		defer C.free(unsafe.Pointer(cRefs[index]))
	}
	var preview C.Preview
	var message *C.char
	if C.resolve_transaction(cScope, cRemote, cOperation, (**C.char)(unsafe.Pointer(&cRefs[0])), C.int(len(cRefs)), &preview, &message) == 0 {
		if message != nil {
			defer C.free(unsafe.Pointer(message))
			return nil, errors.New(C.GoString(message))
		}
		return nil, errors.New("Flatpak could not resolve the transaction")
	}
	changes := make([]Change, int(preview.count))
	for index := range changes {
		op := preview.ops[index]
		changes[index] = Change{Ref: C.GoString(&op.ref[0]), Action: C.GoString(&op.action[0]), DownloadSize: int64(op.download), InstalledSize: int64(op.installed), RequiresAuth: op.auth != 0}
	}
	return changes, nil
}
