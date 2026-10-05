package server

import (
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/auth"
	"github.com/digineo/sitrep/internal/httpx"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/store"
)

var errForbidden = apierr.New(http.StatusForbidden, apierr.Forbidden)

// require answers 403 unless the request's account holds at least role on
// the request's site, or, for requests without site, anywhere.
func (s *Server) require(role model.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acc := auth.Account(r.Context())
		if !acc.Can(r.PathValue("site"), role) {
			httpx.WriteError(w, r, s.log, errForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// actor returns the request's account as the actor of a change that needs
// role on the request's site, or, for requests without site, anywhere.
func actor(r *http.Request, role model.Role) *store.Actor {
	return &store.Actor{
		ID:   auth.Account(r.Context()).ID,
		Site: r.PathValue("site"),
		Role: role,
	}
}

// directory returns the provider's users, or nil if it has no directory.
// A failed reload is logged, and the previous users are returned.
func (s *Server) directory() []string {
	if s.dir == nil {
		return nil
	}

	users, err := s.dir.Users()
	if err != nil {
		s.log.Error("reloading the auth provider's users failed, using the previous ones",
			xlog.Error(err))
	}
	if users == nil {
		return []string{} // an empty directory, unlike none
	}
	return users
}

// parseLogin checks the login naming an account: a user of the provider's
// directory, or else an email address. It reports whether it is an email.
func (s *Server) parseLogin(login string) (string, bool, error) {
	var f apierr.Fields
	if s.dir != nil {
		if !slices.Contains(s.directory(), login) {
			f.Add("login", apierr.UnknownUser)
		}
		return login, false, f.Err()
	}

	email, ok := auth.ParseEmail(strings.TrimSpace(login))
	if !ok {
		f.Add("login", apierr.InvalidEmail)
	}
	return email, true, f.Err()
}

// roles returns a copy of an account's roles, for logRoles.
func roles(a *model.Account) model.Account {
	return model.Account{
		ID:    a.ID,
		Role:  a.Role,
		Sites: maps.Clone(a.Sites),
	}
}

// logRoles records a change of an account's roles by the request's
// account.
func (s *Server) logRoles(r *http.Request, old, acc model.Account) {
	s.log.Info("changed the roles of an account",
		slog.String("by", auth.Account(r.Context()).Subject),
		slog.String("account", acc.ID),
		slog.Group("from",
			slog.String("role", string(old.Role)),
			slog.Any("sites", old.Sites)),
		slog.Group("to",
			slog.String("role", string(acc.Role)),
			slog.Any("sites", acc.Sites)))
}

// member is an account with a role on a site, as the site's maintainers
// see it.
type member struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	// Login is the email, or the subject for providers with a directory.
	Login   string     `json:"login"`
	Role    model.Role `json:"role"`
	Pending bool       `json:"pending"`
}

func (s *Server) newMember(a model.Account, site string) member {
	m := member{
		ID:          a.ID,
		DisplayName: a.DisplayName,
		Login:       a.Email,
		Role:        a.Sites[site],
		Pending:     a.Subject == "",
	}
	if s.dir != nil {
		m.Login = a.Subject
	}
	return m
}

// listMembers lists the accounts of the active provider with a role on
// the site. Admins and owners are not listed: they hold every site role.
func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	if _, err := s.db.Site(site); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	accounts, err := s.db.Accounts()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	provider := auth.Account(r.Context()).Provider
	members := []member{}
	for _, a := range accounts {
		if _, ok := a.Sites[site]; ok && a.Provider == provider {
			members = append(members, s.newMember(a, site))
		}
	}
	httpx.WriteJSON(w, http.StatusOK, members)
}

type grantInput struct {
	Login string     `json:"login"`
	Role  model.Role `json:"role"`
}

// grant reads a login and a role, one of valid, from the request, and
// applies the role by the actor to the login's account, creating the
// account if needed.
func (s *Server) grant(
	w http.ResponseWriter,
	r *http.Request,
	by *store.Actor,
	valid []model.Role,
	apply func(a *model.Account, role model.Role),
) (model.Account, error) {
	var in grantInput
	if err := httpx.ReadJSON(w, r, &in); err != nil {
		return model.Account{}, err
	}

	login, byEmail, err := s.parseLogin(in.Login)
	if err != nil {
		return model.Account{}, err
	}
	if !slices.Contains(valid, in.Role) {
		return model.Account{}, apierr.Fields{{Path: "role", Code: apierr.InvalidValue}}.Err()
	}

	var old model.Account
	provider := auth.Account(r.Context()).Provider
	acc, err := s.db.Grant(by, provider, login, byEmail, func(a *model.Account) error {
		old = roles(a)
		apply(a, in.Role)
		return nil
	})
	if err != nil {
		return model.Account{}, err
	}

	s.logRoles(r, old, acc)
	return acc, nil
}

// addMember gives the account with the login a role on the site, creating
// the account if needed.
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	by := actor(r, model.RoleMaintainer)
	acc, err := s.grant(w, r, by, model.SiteRoles, func(a *model.Account, role model.Role) {
		if a.Sites == nil {
			a.Sites = map[string]model.Role{}
		}
		a.Sites[site] = role
	})
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, s.newMember(acc, site))
}

// putMember changes a member's role on the site.
func (s *Server) putMember(w http.ResponseWriter, r *http.Request) {
	site := r.PathValue("site")
	var in grantInput
	err := httpx.ReadJSON(w, r, &in)
	if err == nil && !slices.Contains(model.SiteRoles, in.Role) {
		err = apierr.Fields{{Path: "role", Code: apierr.InvalidValue}}.Err()
	}

	var old, acc model.Account
	if err == nil {
		acc, err = s.changeMember(r, func(a *model.Account) {
			old = roles(a)
			a.Sites[site] = in.Role
		})
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.logRoles(r, old, acc)
	httpx.WriteJSON(w, http.StatusOK, s.newMember(acc, site))
}

// changeMember applies change to the request's account, if it is a member
// of the request's site other than the requesting one.
func (s *Server) changeMember(
	r *http.Request,
	change func(*model.Account),
) (model.Account, error) {
	by := actor(r, model.RoleMaintainer)
	return s.db.UpdateAccount(by, r.PathValue("account"), func(a *model.Account) error {
		if _, ok := a.Sites[r.PathValue("site")]; !ok {
			return apierr.New(http.StatusNotFound, apierr.NotFound)
		}

		change(a)
		return nil
	})
}

// deleteMember removes an account's role on the site.
func (s *Server) deleteMember(w http.ResponseWriter, r *http.Request) {
	var old model.Account
	acc, err := s.changeMember(r, func(a *model.Account) {
		old = roles(a)
		delete(a.Sites, r.PathValue("site"))
	})
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.logRoles(r, old, acc)
	w.WriteHeader(http.StatusNoContent)
}

// accountView is an account as owners see it. Stale accounts cannot sign
// in: they belong to another provider, or the provider's directory no
// longer lists them.
type accountView struct {
	model.Account
	Pending bool `json:"pending"`
	Stale   bool `json:"stale"`
}

func newAccountView(a model.Account, provider string, users []string) accountView {
	return accountView{
		Account: a,
		Pending: a.Subject == "",
		Stale: a.Provider != provider ||
			users != nil && a.Subject != "" && !slices.Contains(users, a.Subject),
	}
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.db.Accounts()
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	provider := auth.Account(r.Context()).Provider
	users := s.directory()
	views := []accountView{}
	for _, a := range accounts {
		views = append(views, newAccountView(a, provider, users))
	}
	httpx.WriteJSON(w, http.StatusOK, views)
}

// createAccount gives the account with the login a role on the instance,
// creating the account if needed. The role can't be none: an account
// without role would only keep someone's email.
func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	by := actor(r, model.RoleOwner)
	valid := []model.Role{model.RoleAdmin, model.RoleOwner}
	acc, err := s.grant(w, r, by, valid, func(a *model.Account, role model.Role) {
		a.Role = role
	})
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	provider := auth.Account(r.Context()).Provider
	httpx.WriteJSON(w, http.StatusOK, newAccountView(acc, provider, s.directory()))
}

type accountInput struct {
	Role  model.Role            `json:"role"`
	Sites map[string]model.Role `json:"sites"`
}

// putAccount replaces an account's roles on the instance and on sites.
func (s *Server) putAccount(w http.ResponseWriter, r *http.Request) {
	var in accountInput
	err := httpx.ReadJSON(w, r, &in)
	if err == nil {
		err = validateRoles(in)
	}

	var old, acc model.Account
	if err == nil {
		by := actor(r, model.RoleOwner)
		acc, err = s.db.UpdateAccount(by, r.PathValue("account"), func(a *model.Account) error {
			old = roles(a)
			a.Role = in.Role
			a.Sites = in.Sites
			return nil
		})
	}
	if err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.logRoles(r, old, acc)
	provider := auth.Account(r.Context()).Provider
	httpx.WriteJSON(w, http.StatusOK, newAccountView(acc, provider, s.directory()))
}

// validateRoles checks that the roles exist. The store checks that the
// sites do.
func validateRoles(in accountInput) error {
	var f apierr.Fields
	if !slices.Contains(model.InstanceRoles, in.Role) {
		f.Add("role", apierr.InvalidValue)
	}

	for site, role := range in.Sites {
		if !slices.Contains(model.SiteRoles, role) {
			f.Add("sites."+site, apierr.InvalidValue)
		}
	}
	return f.Err()
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("account")
	if err := s.db.DeleteAccount(actor(r, model.RoleOwner), id); err != nil {
		httpx.WriteError(w, r, s.log, err)
		return
	}

	s.log.Info("deleted an account",
		slog.String("by", auth.Account(r.Context()).Subject),
		slog.String("account", id))
	w.WriteHeader(http.StatusNoContent)
}

// getDirectory lists the users of the provider's directory.
func (s *Server) getDirectory(w http.ResponseWriter, r *http.Request) {
	if s.dir == nil {
		httpx.WriteError(w, r, s.log, apierr.New(http.StatusNotFound, apierr.NotFound))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, s.directory())
}
