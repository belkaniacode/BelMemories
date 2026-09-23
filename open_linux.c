// Opening files/folders through GTK so the launched app (file manager, CSV
// viewer) gets an XDG activation token and may raise its window on Wayland.
#include <gtk/gtk.h>
#include "_cgo_export.h"

typedef struct {
	char *uri;
	long id;
} bmOpenReq;

static gboolean bm_show_uri(gpointer data) {
	bmOpenReq *r = data;
	GtkWindow *win = NULL;
	GList *tops = gtk_window_list_toplevels();
	for (GList *l = tops; l != NULL; l = l->next) {
		if (gtk_window_is_active(GTK_WINDOW(l->data))) {
			win = GTK_WINDOW(l->data);
			break;
		}
	}
	if (win == NULL && tops != NULL) {
		win = GTK_WINDOW(tops->data);
	}
	g_list_free(tops);

	GError *err = NULL;
	gboolean ok = gtk_show_uri_on_window(win, r->uri, GDK_CURRENT_TIME, &err);
	bmOpenDone(r->id, ok ? NULL : (char *)(err != NULL ? err->message : "unknown error"));
	if (err != NULL) {
		g_error_free(err);
	}
	g_free(r->uri);
	g_free(r);
	return G_SOURCE_REMOVE;
}

// bm_open_uri schedules the open on the GTK main loop (thread-safe).
void bm_open_uri(const char *uri, long id) {
	bmOpenReq *r = g_new0(bmOpenReq, 1);
	r->uri = g_strdup(uri);
	r->id = id;
	g_idle_add(bm_show_uri, r);
}
