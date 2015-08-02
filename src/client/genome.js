(function() {

// protein: carrier of information and matter.
// Codon, amino acids: Roman character
var Signal = {
	// Intrinsic signals.
	GROWTH: 'g',

	// Transcription modifiers.
	HALF: 'p',
	INVERT: 'i',

	// Intrisinc functionals.
	CHLOROPLAST: 'chlr',
	G_DX: 'x',
	G_DY: 'y',
	G_DZ: 'z',
	DIFF: 'd', // comound: d12 (1:initial signal 2:locator)
	REMOVER: 'r',  // compound: r[1] ([1]: signal to be removed)

	// Positional modifiers.
	CONICAL: 'c',
	HALF_CONICAL: 'h',
	FLIP: 'f',
	TWIST: 't',

	// Cell types.
	LEAF: 'l',
	SHOOT: 's',
	SHOOT_END: 'a',
	FLOWER: 'w',

	// Compounds
	DIFF_SHM: 'dac',
	DIFF_SHS: 'dah',
	DIFF_LF: 'dlf',
};

var parseIntrinsicSignal = function(sig) {
	var inv_table = {};
	_.each(Signal, function(signal, name) {
		inv_table[signal] = name;
	});

	var signals_simple = [Signal.GROWTH,
		Signal.HALF, Signal.CHLOROPLAST,
		Signal.G_DX, Signal.G_DY, Signal.G_DZ];

	var signals_standard = [Signal.LEAF,
		Signal.SHOOT, Signal.SHOOT_END, Signal.FLOWER];

	if(_.contains(signals_simple, sig)) {
		return {
			long: inv_table[sig],
			raw: sig,
			type: 'intrinsic'
		};
	} else if(_.contains(signals_standard, sig)) {
		return {
			long: inv_table[sig],
			raw: sig,
			type: 'standard'
		};
	} else if(sig[0] === Signal.DIFF) {
		if(sig.length === 3) {
			return {
				long: 'DIFF(' + sig[1] + ',' + sig[2] + ')',
				raw: sig,
				type: 'compound'
			};
		} else {
			return {
				long: 'DIFF(?)',
				raw: sig,
				type: 'unknown'
			};
		}
	} else if(sig[0] === Signal.INVERT) {
		if(sig.length >= 2) {
			return {
				long: '!' + sig.substr(1),
				raw: sig,
				type: 'compound'
			};
		} else {
			return {
				long: '!',
				raw: sig,
				type: 'unknown'
			};
		}
	} else if(sig[0] === Signal.REMOVER) {
		if(sig.length >= 2) {
			return {
				long: 'DEL(' + sig.substr(1) + ')',
				raw: sig,
				type: 'compound'
			};
		} else {
			return {
				long: 'DEL',
				raw: sig,
				type: 'unknown'
			};
		}
	} else {
		return {
			long: '',
			raw: sig,
			type: 'unknown'
		};
	}
};


var Genome = function() {
	this.unity = [
		// Topological
		{
			"tracer_desc": "Diff: Produce leaf.",
			"when": [
				Signal.SHOOT_END, Signal.GROWTH, Signal.HALF, Signal.HALF, Signal.HALF
				],
			"emit": [
				Signal.SHOOT, Signal.DIFF_SHM, Signal.DIFF_LF,
				Signal.REMOVER + Signal.SHOOT_END
				]
		},
		{
			"tracer_desc": "Diff: Produce branch.",
			"when": [
				Signal.SHOOT_END, Signal.GROWTH, Signal.HALF, Signal.HALF],
			"emit": [
				Signal.SHOOT, Signal.DIFF_SHM, Signal.DIFF_SHS,
				Signal.REMOVER + Signal.SHOOT_END
				]
		},
		{
			"tracer_desc": "Diff: Produce flower.",
			"when": [
				Signal.SHOOT_END, Signal.INVERT + Signal.GROWTH, Signal.HALF, Signal.HALF],
			"emit": [
				Signal.FLOWER,
				Signal.REMOVER + Signal.SHOOT_END]
		},
		// Growth
		{
			"tracer_desc": "Flower growth.",
			"when": [
				Signal.FLOWER, Signal.HALF, Signal.HALF],
			"emit": [
				Signal.G_DX, Signal.G_DY, Signal.G_DZ]
		},
		{
			"tracer_desc": "Leaf elongation.",
			"when": [
				Signal.LEAF],
			"emit": [
				Signal.G_DZ],
		},
		{
			"tracer_desc": "Leaf shape adjustment.",
			"when": [
				Signal.LEAF, Signal.HALF, Signal.HALF, Signal.HALF, Signal.HALF],
			"emit": [
				Signal.G_DX, Signal.G_DX, Signal.G_DX, Signal.G_DX, Signal.G_DX, Signal.G_DX, Signal.G_DY]
		},
		{
			"tracer_desc": "Shoot (end) elongation.",
			"when": [
				Signal.SHOOT],
			"emit": [Signal.G_DZ]
		},
		{
			"tracer_desc": "Shoot thickening.",
			"when": [
				Signal.SHOOT_END, Signal.HALF, Signal.HALF, Signal.HALF],
			"emit": [Signal.G_DX, Signal.G_DY]
		},
		{
			"tracer_desc": "Chloroplast generation.",
			"when": [
				Signal.LEAF, Signal.HALF, Signal.HALF, Signal.HALF],
			"emit": [
				Signal.CHLOROPLAST]
		}
	];
};

// Clone "naturally" with mutations.
// Since real bio is too complex, use a simple rule that can
// diffuse into all states.
// return :: Genome
Genome.prototype.naturalClone = function() {
	var _this = this;

	var genome = new Genome();
	genome.unity = this._shuffle(this.unity,
		function(gene) {
			return _this._naturalCloneGene(gene, '');
		},
		function(gene) {
			return _this._naturalCloneGene(gene, '/Duplicated/');
		});

	return genome;
};

// flag :: A text to attach to description tracer.
// return :: Genome.gene
Genome.prototype._naturalCloneGene = function(gene_old, flag) {
	var _this = this;

	var gene = {};
	
	gene["when"] = this._shuffle(gene_old["when"],
		function(sig) { return _this._naturalCloneSignal(sig); },
		function(sig) { return _this._naturalCloneSignal(sig); });
	gene["emit"] = this._shuffle(gene_old["emit"],
		function(sig) { return _this._naturalCloneSignal(sig); },
		function(sig) { return _this._naturalCloneSignal(sig); });

	gene["tracer_desc"] = flag + gene_old["tracer_desc"];
	return gene;
};


Genome.prototype._naturalCloneSignal = function(sig) {
	function random_sig1() {
		var set = 'abcdefghijklmnopqrstuvwxyz';
		return set[Math.floor(Math.random() * set.length)];
	}

	var new_sig = '';
	for(var i = 0; i < sig.length; i++) {
		if(Math.random() > 0.01) {
			new_sig += sig[i];
		}
		if(Math.random() < 0.01) {
			new_sig += random_sig1();
		}
	}
	return new_sig;
};

Genome.prototype._shuffle = function(array, modifier_normal, modifier_dup) {
	var result = [];

	// 1st pass: Copy with occasional misses.
	_.each(array, function(elem) {
		if(Math.random() > 0.01) {
			result.push(modifier_normal(elem));
		}
	});

	// 2nd pass: Occasional duplications.
	_.each(array, function(elem) {
		if(Math.random() < 0.01) {
			result.push(modifier_dup(elem));
		}
	});

	return result;
};

// xs :: [num]
// return :: num
function sum(xs) {
	return _.reduce(xs, function(x, y) {
		return x + y;
	}, 0);
}

this.Genome = Genome;
this.parseIntrinsicSignal = parseIntrinsicSignal;
this.Signal = Signal;

})(this);