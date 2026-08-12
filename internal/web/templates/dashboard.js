document.addEventListener('DOMContentLoaded', () => {
    const E = ExpenseOwl;
    const palette = ['#d7b46a','#76c7b7','#75a9d6','#e47c74','#9a8ec7','#d3996d','#72b88a','#c47f9b','#8ea7b9','#b8ad79'];
    let date = new Date();
    let expenses = [];
    let config = { categories: [], categoryTargets: {} };
    let getOwner = () => E.savedOwner();
    let pieChart;
    let trendChart;
    const disabled = new Set();

    const monthElement = document.getElementById('currentMonth');
    const drawer = document.getElementById('transactionDrawer');
    const form = document.getElementById('transactionForm');
    document.getElementById('date').value = E.localDateISO();

    getOwner = E.bindOwnerRail(document.getElementById('ownerRail'), render);
    document.getElementById('prevMonth').addEventListener('click', () => moveMonth(-1));
    document.getElementById('nextMonth').addEventListener('click', () => moveMonth(1));
    document.getElementById('toggleForm').addEventListener('click', () => drawer.classList.toggle('open'));
    document.getElementById('closeForm').addEventListener('click', () => drawer.classList.remove('open'));

    async function moveMonth(offset) {
        date = new Date(date.getFullYear(), date.getMonth() + offset, 15);
        disabled.clear();
        await loadExpenses();
        render();
    }

    async function loadExpenses() {
        const from = new Date(date.getFullYear(), date.getMonth() - 11, 1);
        const to = new Date(date.getFullYear(), date.getMonth() + 1, 1);
        expenses = await E.request(`/expenses?from=${encodeURIComponent(from.toISOString())}&to=${encodeURIComponent(to.toISOString())}`);
    }

    function visibleExpenses() { return E.ownerExpenses(expenses, getOwner()); }
    function monthExpenses(month = date) { return visibleExpenses().filter(expense => E.inMonth(expense, month)); }

    function render() {
        monthElement.textContent = E.monthLabel(date);
        const current = monthExpenses();
        const income = current.filter(item => item.amount > 0).reduce((sum, item) => sum + item.amount, 0);
        const spending = current.filter(item => item.amount < 0).reduce((sum, item) => sum + Math.abs(item.amount), 0);
        const balance = income - spending;
        document.getElementById('income').textContent = E.euro(income);
        document.getElementById('spending').textContent = E.euro(spending);
        const balanceElement = document.getElementById('balance');
        balanceElement.textContent = E.euro(balance);
        balanceElement.className = `metric-value ${balance >= 0 ? 'positive' : 'negative'}`;
        renderPie(current);
        renderTargets(current);
        renderTrend();
    }

    function categoryBreakdown(current) {
        const totals = new Map();
        current.filter(item => item.amount < 0 && !disabled.has(item.category)).forEach(item => totals.set(item.category, (totals.get(item.category) || 0) + Math.abs(item.amount)));
        return [...totals].sort((a, b) => b[1] - a[1]);
    }

    function categoryColor(category) {
        const index = config.categories.indexOf(category);
        if (index >= 0) return palette[index % palette.length];
        const hash = [...category].reduce((value, character) => value + character.charCodeAt(0), 0);
        return palette[hash % palette.length];
    }

    function renderPie(current) {
        const allCategories = [...new Set(current.filter(item => item.amount < 0).map(item => item.category))];
        const breakdown = categoryBreakdown(current);
        document.getElementById('chartContent').hidden = allCategories.length === 0;
        document.getElementById('chartEmpty').hidden = allCategories.length > 0;
        if (pieChart) pieChart.destroy();
        if (breakdown.length) pieChart = new Chart(document.getElementById('categoryChart'), { type: 'doughnut', data: { labels: breakdown.map(item => item[0]), datasets: [{ data: breakdown.map(item => item[1]), backgroundColor: breakdown.map(item => categoryColor(item[0])), borderColor: '#111821', borderWidth: 3 }] }, options: { responsive: true, maintainAspectRatio: false, cutout: '66%', plugins: { legend: { display: false }, tooltip: { callbacks: { label: context => `${context.label}: ${E.euro(context.raw)}` } } } } });
        const totals = new Map(breakdown);
        const grandTotal = breakdown.reduce((sum, item) => sum + item[1], 0);
        document.getElementById('categoryLegend').innerHTML = allCategories.sort((a, b) => (totals.get(b) || 0) - (totals.get(a) || 0)).map(category => `<button class="legend-row ${disabled.has(category) ? 'disabled' : ''}" data-category="${E.escape(category)}"><span class="swatch" style="background:${categoryColor(category)}"></span><span>${E.escape(category)}</span><span class="money">${totals.has(category) ? E.euro(totals.get(category)) : 'Excluded'}</span></button>`).join('') + (allCategories.length ? `<div class="legend-row"><span></span><strong>Total</strong><strong class="money">${E.euro(grandTotal)}</strong></div>` : '');
    }

    document.getElementById('categoryLegend').addEventListener('click', event => {
        const row = event.target.closest('[data-category]');
        if (!row) return;
        disabled.has(row.dataset.category) ? disabled.delete(row.dataset.category) : disabled.add(row.dataset.category);
        renderPie(monthExpenses());
    });

    function renderTargets(current) {
        const targets = Object.entries(config.categoryTargets || {}).filter(([, target]) => target > 0);
        const section = document.getElementById('targetsSection');
        section.hidden = !targets.length;
        document.getElementById('targetsGrid').innerHTML = targets.map(([category, target]) => {
            const spent = current.filter(item => item.amount < 0 && item.category === category).reduce((sum, item) => sum + Math.abs(item.amount), 0);
            const percent = Math.round(spent / target * 100);
            return `<article class="target-card"><div class="target-line"><strong>${E.escape(category)}</strong><span class="money">${percent}%</span></div><div class="target-track ${percent > 100 ? 'over' : ''}"><span style="width:${Math.min(percent, 100)}%"></span></div><div class="target-foot"><span>${E.euro(spent)} spent</span><span>${E.euro(target)} target</span></div></article>`;
        }).join('');
    }

    function renderTrend() {
        const periods = Array.from({ length: 12 }, (_, index) => new Date(date.getFullYear(), date.getMonth() - 11 + index, 15));
        const data = periods.map(period => {
            const items = monthExpenses(period);
            return { income: items.filter(item => item.amount > 0).reduce((sum, item) => sum + item.amount, 0), spending: items.filter(item => item.amount < 0).reduce((sum, item) => sum + Math.abs(item.amount), 0) };
        });
        if (trendChart) trendChart.destroy();
        trendChart = new Chart(document.getElementById('trendChart'), { type: 'line', data: { labels: periods.map(period => period.toLocaleDateString(undefined, { month: 'short', year: '2-digit' })), datasets: [{ label: 'Income', data: data.map(item => item.income), borderColor: '#76c7b7', backgroundColor: 'rgba(118,199,183,.08)', fill: true, tension: .3 }, { label: 'Spending', data: data.map(item => item.spending), borderColor: '#e47c74', backgroundColor: 'rgba(228,124,116,.06)', fill: true, tension: .3 }] }, options: { responsive: true, maintainAspectRatio: false, interaction: { intersect: false, mode: 'index' }, scales: { y: { beginAtZero: true, ticks: { callback: value => E.euro(value) } } } } });
    }

    form.addEventListener('submit', async event => {
        event.preventDefault();
        const button = form.querySelector('[type=submit]');
        const message = document.getElementById('formMessage');
        button.disabled = true;
        try {
            E.setMessage(message, 'Saving…');
            const receipt = await E.uploadReceipt(document.getElementById('receipt').files[0]);
            let amount = Number(document.getElementById('amount').value);
            if (!document.getElementById('reportGain').checked) amount *= -1;
            await E.request('/expense', E.json('PUT', { name: document.getElementById('name').value, category: document.getElementById('category').value, amount, date: E.dateInputToISO(document.getElementById('date').value), owner: getOwner(), notes: document.getElementById('notes').value, receipt }));
            form.reset(); document.getElementById('date').value = E.localDateISO();
            E.setMessage(message, 'Transaction saved.', 'success');
            await loadExpenses(); render();
        } catch (error) { E.setMessage(message, error.message, 'error'); }
        finally { button.disabled = false; }
    });

    (async () => {
        try {
            [config] = await Promise.all([E.request('/config'), loadExpenses()]);
            document.getElementById('category').innerHTML = config.categories.map(category => `<option>${E.escape(category)}</option>`).join('');
            Chart.defaults.color = '#92a0ae'; Chart.defaults.borderColor = '#2b3642'; Chart.defaults.font.family = 'Inter, system-ui, sans-serif';
            render();
        } catch (error) { document.getElementById('chartEmpty').hidden = false; document.getElementById('chartEmpty').textContent = error.message; }
    })();
});
