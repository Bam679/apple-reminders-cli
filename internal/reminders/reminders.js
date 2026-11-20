// Static EventKit bridge for Apple Reminders. Never interpolate input into
// this file: all input arrives as decoded JSON in `a` (see osa.Prelude).
ObjC.import('EventKit');

const ENTITY_REMINDER = 1;
const FULL_ACCESS = 3;
const UNITS_DAY = 4 | 8 | 16;          // year | month | day
const UNITS_MINUTE = UNITS_DAY | 32 | 64; // + hour | minute

function isNil(x) {
  return x === undefined || x === null || (typeof x.isNil === 'function' && x.isNil());
}
function str(x) { return isNil(x) ? '' : ObjC.unwrap(x); }
function ms(d) { return isNil(d) ? null : Math.round(d.timeIntervalSince1970 * 1000); }

function waitFor(state, what) {
  const t0 = Date.now();
  while (!state.done) {
    if (Date.now() - t0 > 30000) fail('unavailable', 'timed out ' + what);
    $.NSRunLoop.currentRunLoop.runUntilDate($.NSDate.dateWithTimeIntervalSinceNow(0.01));
  }
}

function store() {
  let status = Number($.EKEventStore.authorizationStatusForEntityType(ENTITY_REMINDER));
  const s = $.EKEventStore.alloc.init;
  if (status === 0) {
    const st = { done: false };
    s.requestFullAccessToRemindersWithCompletion(function () { st.done = true; });
    waitFor(st, 'waiting for Reminders permission');
    status = Number($.EKEventStore.authorizationStatusForEntityType(ENTITY_REMINDER));
  }
  if (status !== FULL_ACCESS) {
    fail('permission', 'Reminders access not granted (status ' + status + '); enable your terminal app under Privacy & Security > Reminders');
  }
  return s;
}

function listOut(c) {
  return { id: str(c.calendarIdentifier), title: str(c.title), account: str(c.source.title), writable: !!c.allowsContentModifications };
}

function reminderOut(r) {
  const o = {
    id: str(r.calendarItemIdentifier),
    title: str(r.title),
    list: str(r.calendar.title),
    list_id: str(r.calendar.calendarIdentifier),
    completed: !!r.completed,
    completed_ms: ms(r.completionDate),
    priority: Number(r.priority),
    notes: str(r.notes),
    url: isNil(r.URL) ? '' : str(r.URL.absoluteString),
    created_ms: ms(r.creationDate),
    modified_ms: ms(r.lastModifiedDate),
    due_ms: null,
    due_all_day: false,
  };
  const c = r.dueDateComponents;
  if (!isNil(c)) {
    o.due_ms = ms($.NSCalendar.currentCalendar.dateFromComponents(c));
    o.due_all_day = Number(c.hour) > 100000; // NSDateComponentUndefined
  }
  return o;
}

function calendars(s) {
  const out = [];
  const cs = s.calendarsForEntityType(ENTITY_REMINDER);
  for (let i = 0; i < cs.count; i++) out.push(cs.objectAtIndex(i));
  return out;
}

function calendarById(s, id) {
  const c = s.calendarWithIdentifier(id);
  if (isNil(c)) fail('not_found', 'no reminder list with id ' + id);
  return c;
}

function fetch(s, pred) {
  const st = { done: false, res: null };
  s.fetchRemindersMatchingPredicateCompletion(pred, function (r) { st.res = r; st.done = true; });
  waitFor(st, 'fetching reminders');
  const out = [];
  if (isNil(st.res)) return out;
  for (let i = 0; i < st.res.count; i++) out.push(st.res.objectAtIndex(i));
  return out;
}

function reminderById(s, id) {
  const it = s.calendarItemWithIdentifier(id);
  if (isNil(it) || !it.isKindOfClass($.EKReminder)) fail('not_found', 'no reminder with id ' + id);
  return it;
}

function setDue(r, dueMs, allDay) {
  const d = $.NSDate.dateWithTimeIntervalSince1970(dueMs / 1000);
  r.dueDateComponents = $.NSCalendar.currentCalendar.componentsFromDate(allDay ? UNITS_DAY : UNITS_MINUTE, d);
}

function save(s, r) {
  if (!s.saveReminderCommitError(r, true, null)) fail('internal', 'saving the reminder failed');
}

const OPS = {
  lists: function () {
    return calendars(store()).map(listOut);
  },
  reminders: function (a) {
    const s = store();
    let cals = $();
    if (a.list_ids && a.list_ids.length) {
      cals = $.NSMutableArray.array;
      a.list_ids.forEach(function (id) { cals.addObject(calendarById(s, id)); });
    }
    let preds;
    if (a.status === 'completed') preds = [s.predicateForCompletedRemindersWithCompletionDateStartingEndingCalendars($(), $(), cals)];
    else if (a.status === 'all') preds = [s.predicateForRemindersInCalendars(cals)];
    else preds = [s.predicateForIncompleteRemindersWithDueDateStartingEndingCalendars($(), $(), cals)];
    let out = [];
    preds.forEach(function (p) { out = out.concat(fetch(s, p).map(reminderOut)); });
    return out;
  },
  get: function (a) {
    return reminderOut(reminderById(store(), a.id));
  },
  add: function (a) {
    const s = store();
    const r = $.EKReminder.reminderWithEventStore(s);
    r.calendar = calendarById(s, a.list_id);
    r.title = a.title;
    if (a.notes) r.notes = a.notes;
    if (a.url) r.URL = $.NSURL.URLWithString(a.url);
    if (a.priority) r.priority = a.priority;
    if (a.due_ms !== null && a.due_ms !== undefined) setDue(r, a.due_ms, a.due_all_day);
    save(s, r);
    return reminderOut(r);
  },
  update: function (a) {
    const s = store();
    const r = reminderById(s, a.id);
    if (a.title !== null && a.title !== undefined) r.title = a.title;
    if (a.notes !== null && a.notes !== undefined) r.notes = a.notes;
    if (a.priority !== null && a.priority !== undefined) r.priority = a.priority;
    if (a.clear_due) r.dueDateComponents = $();
    else if (a.due_ms !== null && a.due_ms !== undefined) setDue(r, a.due_ms, a.due_all_day);
    save(s, r);
    return reminderOut(r);
  },
  set_completed: function (a) {
    const s = store();
    const r = reminderById(s, a.id);
    r.completed = !!a.completed;
    save(s, r);
    return reminderOut(r);
  },
  delete: function (a) {
    const s = store();
    const r = reminderById(s, a.id);
    if (!s.removeReminderCommitError(r, true, null)) fail('internal', 'deleting the reminder failed');
    return { id: a.id };
  },
  list_create: function (a) {
    const s = store();
    const c = $.EKCalendar.calendarForEntityTypeEventStore(ENTITY_REMINDER, s);
    c.title = a.title;
    const def = s.defaultCalendarForNewReminders;
    if (isNil(def)) fail('unavailable', 'no default reminders account');
    c.source = def.source;
    if (!s.saveCalendarCommitError(c, true, null)) fail('internal', 'creating the list failed');
    return listOut(c);
  },
  list_delete: function (a) {
    const s = store();
    const c = calendarById(s, a.id);
    if (!a.force) {
      const arr = $.NSMutableArray.array;
      arr.addObject(c);
      if (fetch(s, s.predicateForRemindersInCalendars(arr)).length > 0) fail('conflict', 'list is not empty; delete its reminders first');
    }
    if (!s.removeCalendarCommitError(c, true, null)) fail('internal', 'deleting the list failed');
    return { id: a.id };
  },
};
