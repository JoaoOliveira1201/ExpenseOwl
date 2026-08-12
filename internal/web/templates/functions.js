const ExpenseOwl = (() => {
    const owners = ['joao', 'maria', 'common'];

    function euro(value) {
        return new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' }).format(Number(value) || 0);
    }

    function monthLabel(date) {
        return date.toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
    }

    function monthBounds(date) {
        const start = new Date(date.getFullYear(), date.getMonth(), 1);
        const end = new Date(date.getFullYear(), date.getMonth() + 1, 1);
        return { start, end };
    }

    function inMonth(expense, date) {
        const { start, end } = monthBounds(date);
        const value = new Date(expense.date);
        return value >= start && value < end;
    }

    function ownerExpenses(expenses, owner) {
        if (owner === 'common') return expenses;
        return expenses.filter(expense => (expense.owner || 'common') === owner);
    }

    function savedOwner() {
        const owner = localStorage.getItem('expenseowl_owner');
        return owners.includes(owner) ? owner : 'joao';
    }

    function saveOwner(owner) {
        localStorage.setItem('expenseowl_owner', owner);
    }

    function ownerLabel(owner) {
        return { joao: 'João', maria: 'Maria', common: 'Together' }[owner] || 'Together';
    }

    function bindOwnerRail(element, onChange) {
        let owner = savedOwner();
        const render = () => element.querySelectorAll('[data-owner]').forEach(button => {
            button.classList.toggle('active', button.dataset.owner === owner);
            button.setAttribute('aria-pressed', String(button.dataset.owner === owner));
        });
        element.addEventListener('click', event => {
            const button = event.target.closest('[data-owner]');
            if (!button || button.dataset.owner === owner) return;
            owner = button.dataset.owner;
            saveOwner(owner);
            render();
            onChange(owner);
        });
        render();
        return () => owner;
    }

    async function request(url, options = {}) {
        const response = await fetch(url, options);
        if (response.status === 204) return null;
        const contentType = response.headers.get('content-type') || '';
        const data = contentType.includes('json') ? await response.json() : await response.text();
        if (!response.ok) throw new Error(data?.error || data || `Request failed (${response.status})`);
        return data;
    }

    function json(method, body) {
        return { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
    }

    function localDateISO(date = new Date()) {
        const offset = date.getTimezoneOffset();
        return new Date(date.getTime() - offset * 60000).toISOString().slice(0, 10);
    }

    function dateInputToISO(value) {
        const [year, month, day] = value.split('-').map(Number);
        const now = new Date();
        return new Date(year, month - 1, day, now.getHours(), now.getMinutes(), 0).toISOString();
    }

    function dateTime(value) {
        return new Date(value).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
    }

    function escape(value) {
        const node = document.createElement('span');
        node.textContent = value == null ? '' : String(value);
        return node.innerHTML;
    }

    function setMessage(element, message = '', type = '') {
        element.textContent = message;
        element.className = `form-message ${type}`.trim();
    }

    async function uploadReceipt(file) {
        if (!file) return '';
        const data = new FormData();
        data.append('receipt', file);
        const result = await request('/receipt/upload', { method: 'POST', body: data });
        return result.receipt;
    }

    return { euro, monthLabel, monthBounds, inMonth, ownerExpenses, savedOwner, ownerLabel, bindOwnerRail, request, json, localDateISO, dateInputToISO, dateTime, escape, setMessage, uploadReceipt };
})();
